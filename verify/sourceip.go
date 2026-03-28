// sourceip.go — Proves SNAT is disabled: each US container's real IP appears
// as the source in MySQL's processlist, then live-tails the general query log
// on eu-db to show connections color-coded by source container.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	yellow  = "\033[0;33m"
	magenta = "\033[0;35m"
)

func sourceIPCmd() *cobra.Command {
	var interval time.Duration

	cmd := &cobra.Command{
		Use:   "source-ip",
		Short: "Verify viewers show distinct source IPs in MySQL, then live-tail connections",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSourceIPCheck(interval)
		},
	}

	cmd.Flags().DurationVarP(&interval, "interval", "i", 500*time.Millisecond, "Poll interval for live tail (e.g. 500ms, 1s)")
	return cmd
}

func runSourceIPCheck(interval time.Duration) error {
	passed, failed := 0, 0
	p := func(msg string) { passed++; pass(msg) }
	f := func(msg string) { failed++; fail(msg) }

	// ── Discover bind address ────────────────────────────────────────
	header("MySQL Configuration (eu-db)")
	bindAddr := discoverBindAddr()
	if bindAddr == "" {
		f("Could not discover MySQL bind address from eu-db")
		return nil
	}
	info(fmt.Sprintf("MySQL bind address: %s", bindAddr))

	// ── Source IP Differentiation ────────────────────────────────────
	header("Source IP Differentiation — us-app containers → eu-db (SNAT disabled)")

	sourceIPQuery := fmt.Sprintf(
		`timeout 5 mysql -h %s -u app -papppass -D app --skip-ssl -N -e "SELECT HOST FROM information_schema.processlist WHERE ID = CONNECTION_ID()" 2>/dev/null`,
		bindAddr,
	)

	type sourceIPTest struct {
		container string
		label     string
	}

	sourceIPTests := []sourceIPTest{
		{"us-euro-viewer", "us-euro-viewer (172.21.0.10)"},
		{"us-euro-viewer-admin", "us-euro-viewer-admin (172.21.0.20)"},
	}

	sourceIPs := make(map[string]string)

	for _, s := range sourceIPTests {
		res := dockerExecTimeout(s.container, sourceIPQuery, 10*time.Second)
		if !res.OK() || strings.TrimSpace(res.Stdout) == "" {
			f(fmt.Sprintf("%s — could not query source IP", s.label))
			continue
		}
		host := strings.TrimSpace(res.Stdout)
		if idx := strings.LastIndex(host, ":"); idx > 0 {
			host = host[:idx]
		}
		sourceIPs[s.container] = host
		info(fmt.Sprintf("%s → MySQL sees source IP: %s", s.label, host))
	}

	viewerIP := sourceIPs["us-euro-viewer"]
	adminIP := sourceIPs["us-euro-viewer-admin"]

	if viewerIP != "" && adminIP != "" {
		if viewerIP != adminIP {
			p(fmt.Sprintf("Source IPs are distinct (%s ≠ %s) — SNAT disabled, per-container identity preserved", viewerIP, adminIP))
		} else {
			f(fmt.Sprintf("Source IPs are identical (%s) — SNAT may be masquerading container IPs", viewerIP))
		}
	} else {
		f("Could not compare source IPs — one or both queries failed")
	}

	printSummary(passed, failed)

	// ── Live MySQL connection log on eu-db ────────────────────────────
	header("Live MySQL Connection Log")
	fmt.Printf("%s  Tailing /var/log/mysql/general.log on eu-db…%s\n", dim, reset)
	fmt.Printf("%s  Press Ctrl-C to stop%s\n\n", dim, reset)

	ipColor := map[string]string{
		"172.21.0.10": green,
		"172.21.0.20": magenta,
	}
	ipLabel := map[string]string{
		"172.21.0.10": "us-euro-viewer",
		"172.21.0.20": "us-euro-viewer-admin",
	}

	for ip, color := range ipColor {
		fmt.Printf("  %s●%s  %s (%s)\n", color, reset, ipLabel[ip], ip)
	}
	fmt.Println()

	fmt.Printf("  %s%-12s  %-6s  %-24s  %-10s  %s%s\n",
		bold, "TIME", "ID", "SOURCE", "EVENT", "DETAIL", reset)
	fmt.Printf("  %s────────────  ──────  ────────────────────────  ──────────  ─────────────────────%s\n", dim, reset)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	// Get current file size so we only show new entries
	sizeRes := orbRun("eu-db", `stat -c %s /var/log/mysql/general.log 2>/dev/null || echo 0`)
	offset := 0
	if sizeRes.OK() {
		fmt.Sscanf(strings.TrimSpace(sizeRes.Stdout), "%d", &offset)
	}

	for {
		select {
		case <-sigCh:
			fmt.Printf("\n%s%sStopped.%s\n", dim, bold, reset)
			return nil
		default:
		}

		tailCmd := fmt.Sprintf(`sudo tail -c +%d /var/log/mysql/general.log 2>/dev/null`, offset+1)
		res := orbRunTimeout("eu-db", tailCmd, 5*time.Second)

		if res.OK() && len(res.Stdout) > 0 {
			offset += len(res.Stdout)

			for _, line := range strings.Split(res.Stdout, "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}

				if !strings.Contains(line, "Connect") && !strings.Contains(line, "Query") {
					continue
				}
				if strings.Contains(line, "processlist") || strings.Contains(line, "general_log") {
					continue
				}

				ip := ""
				for knownIP := range ipColor {
					if strings.Contains(line, knownIP) {
						ip = knownIP
						break
					}
				}

				fields := strings.Fields(line)
				connID, event, detail, ts := "", "", "", ""

				if len(fields) >= 3 {
					if strings.Contains(fields[0], "T") || strings.Contains(fields[0], "-") {
						ts = fields[0]
						if len(fields) >= 4 {
							connID = fields[1]
							event = fields[2]
							detail = strings.Join(fields[3:], " ")
						}
					} else {
						connID = fields[0]
						event = fields[1]
						detail = strings.Join(fields[2:], " ")
					}
				}

				if event != "Connect" && event != "Query" {
					continue
				}
				if event == "Connect" && !strings.Contains(detail, "app") {
					continue
				}

				displayTime := time.Now().Format("15:04:05.00")
				if ts != "" {
					if t, err := time.Parse("2006-01-02T15:04:05.000000Z", ts); err == nil {
						displayTime = t.Format("15:04:05.00")
					}
				}

				if len(detail) > 60 {
					detail = detail[:60] + "…"
				}

				color := yellow
				label := ""
				if ip != "" {
					if c, ok := ipColor[ip]; ok {
						color = c
						label = ipLabel[ip] + " (" + ip + ")"
					} else {
						label = ip
					}
				}

				if label != "" {
					fmt.Printf("  %s  %s%-6s%s  %s%-24s%s  %-10s  %s\n",
						displayTime, color, connID, reset, color, label, reset, event, detail)
				} else if event == "Query" && (strings.Contains(detail, "famous_europeans") || strings.Contains(detail, "SELECT")) {
					fmt.Printf("  %s  %s%-6s%s  %-24s  %-10s  %s\n",
						displayTime, dim, connID, reset, dim+"—"+reset, event, detail)
				}
			}
		}

		time.Sleep(interval)
	}
}
