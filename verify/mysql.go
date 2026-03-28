// mysql.go — Verifies MySQL access control and connectivity.
// Probes eu-db:3306 from six vantage points (two should be allowed by ACL,
// four should be blocked), then runs a ping sweep from each source.
package main

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func mysqlCmd() *cobra.Command {
	var pingCount int

	cmd := &cobra.Command{
		Use:   "mysql-access",
		Short: "Verify MySQL is inaccessible outside Tailscale and run connectivity ping sweeps",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMySQLChecks(pingCount)
		},
	}

	cmd.Flags().IntVarP(&pingCount, "pings", "n", 10, "Number of pings per source in the sweep")
	return cmd
}

func runMySQLChecks(pingCount int) error {
	passed, failed := 0, 0
	p := func(msg string) { passed++; pass(msg) }
	f := func(msg string) { failed++; fail(msg) }

	// ── Discover MySQL bind address on eu-db ─────────────────────────
	header("MySQL Configuration (eu-db)")

	bindAddr := discoverBindAddr()
	if bindAddr == "" {
		f("Could not read MySQL bind address from eu-db")
		fmt.Printf("\n%s%sAborted — cannot continue without bind address.%s\n", red, bold, reset)
		return nil
	}

	if bindAddr != "0.0.0.0" && bindAddr != "127.0.0.1" {
		p(fmt.Sprintf("MySQL bound to bridge IP: %s (not 0.0.0.0)", bindAddr))
	} else {
		f(fmt.Sprintf("MySQL bound to %s — should be bridge IP", bindAddr))
	}

	// ── Listening sockets ────────────────────────────────────────────
	header("MySQL Listening Sockets (eu-db)")

	listenRes := orbRun("eu-db", `sudo ss -tlnp | grep ':3306'`)
	if listenRes.OK() {
		lines := strings.TrimSpace(listenRes.Stdout)
		if !strings.Contains(lines, "0.0.0.0:3306") {
			p(fmt.Sprintf("MySQL listening on %s:3306 only", bindAddr))
		} else {
			f("MySQL listening on 0.0.0.0:3306 — exposed to all interfaces")
		}
		for _, line := range strings.Split(lines, "\n") {
			info(strings.TrimSpace(line))
		}
	} else {
		f("Could not read listening sockets on eu-db")
	}

	// ── Open ports on eu-db ──────────────────────────────────────────
	header("eu-db Open Ports (expect only MySQL + tailscaled)")

	portsRes := orbRun("eu-db", `sudo ss -tlnp | grep LISTEN`)
	if portsRes.OK() {
		var unexpected []string
		for _, line := range strings.Split(strings.TrimSpace(portsRes.Stdout), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if strings.Contains(line, ":3306 ") || strings.Contains(line, "127.0.0.1:33060") || strings.Contains(line, "tailscaled") {
				continue
			}
			unexpected = append(unexpected, line)
		}
		if len(unexpected) == 0 {
			p("eu-db — only MySQL 3306 and tailscaled ports listening")
		} else {
			f("eu-db — unexpected ports open:")
			for _, u := range unexpected {
				info(u)
			}
		}
	} else {
		f("Could not list ports on eu-db")
	}

	// ── MySQL Access Control ─────────────────────────────────────────
	// Probe eu-db:3306 from six vantage points. The two app containers
	// should be ALLOWED (routed through the Tailscale mesh via the ACL
	// grant). Everything else should be BLOCKED.
	header(fmt.Sprintf("MySQL Access Control — probing eu-db (%s:3306)", bindAddr))

	type accessTest struct {
		name     string           // source → destination shown in output
		run      func() runResult // executes the probe
		expected bool             // true = ALLOWED, false = BLOCKED
	}

	const probeTimeout = 5 * time.Second

	// Each probe is tailored to the tools available in its environment.
	mysqlProbe := fmt.Sprintf(
		`timeout 5 mysql -h %s -u app -papppass -D app --skip-ssl -e 'SELECT 1' 2>&1`,
		bindAddr,
	)
	// Alpine containers (us-app-ts) need mariadb-client installed first
	mysqlProbeAlpine := fmt.Sprintf(
		`apk add --no-cache mariadb-client >/dev/null 2>&1; timeout 5 mariadb -h %s -u app -papppass -D app --skip-ssl -e 'SELECT 1' 2>&1`,
		bindAddr,
	)
	// VMs without mysql client fall back to a raw TCP probe via nc
	mysqlProbeNC := fmt.Sprintf(
		`timeout 5 mysql -h %s -u app -papppass -D app --skip-ssl -e 'SELECT 1' 2>&1 || timeout 5 bash -c 'echo QUIT | nc -w 5 %s 3306' 2>&1`,
		bindAddr, bindAddr,
	)
	// Mac host — TCP probe only
	mysqlProbeMac := fmt.Sprintf(
		`nc -z -w 5 -G 5 %s 3306 2>&1`, bindAddr,
	)

	tests := []accessTest{
		{
			name:     "us-euro-viewer → eu-db:3306",
			run:      func() runResult { return dockerExecTimeout("us-euro-viewer", mysqlProbe, probeTimeout) },
			expected: true,
		},
		{
			name:     "us-euro-viewer-admin → eu-db:3306",
			run:      func() runResult { return dockerExecTimeout("us-euro-viewer-admin", mysqlProbe, probeTimeout) },
			expected: true,
		},
		{
			name:     "us-app-ts (sidecar) → eu-db:3306",
			run:      func() runResult { return dockerExecTimeout("us-app-ts", mysqlProbeAlpine, 15*time.Second) },
			expected: false,
		},
		{
			name:     "eu-router VM → eu-db:3306",
			run:      func() runResult { return orbRunTimeout("eu-router", mysqlProbeNC, probeTimeout) },
			expected: false,
		},
		{
			name:     "us-app VM host → eu-db:3306",
			run:      func() runResult { return orbRunTimeout("us-app", mysqlProbeNC, probeTimeout) },
			expected: false,
		},
		{
			name:     "Mac host (local) → eu-db:3306",
			run:      func() runResult { return localRunTimeout(mysqlProbeMac, probeTimeout) },
			expected: false,
		},
	}

	for _, t := range tests {
		// Run the probe in the background with a live "waiting" ticker
		type result struct {
			res       runResult
			succeeded bool
		}
		ch := make(chan result, 1)
		start := time.Now()

		go func() {
			r := t.run()
			ch <- result{res: r, succeeded: r.OK() && !isBlocked(r)}
		}()

		ticker := time.NewTicker(500 * time.Millisecond)
		fmt.Printf("  %-30s %swaiting…%s", t.name, dim, reset)

		var got result
		done := false
		for !done {
			select {
			case got = <-ch:
				done = true
			case <-ticker.C:
				elapsed := time.Since(start).Seconds()
				fmt.Printf("\r  %-30s %swaiting… %.0fs%s", t.name, dim, elapsed, reset)
			}
		}
		ticker.Stop()
		elapsed := time.Since(start)

		// Clear the waiting line and print result
		fmt.Printf("\r  %-30s ", t.name)
		if t.expected && got.succeeded {
			passed++
			fmt.Printf("%s✔ ALLOWED%s  (query returned — %.1fs)\n", green, reset, elapsed.Seconds())
		} else if t.expected && !got.succeeded {
			failed++
			detail := strings.TrimSpace(got.res.Stderr + " " + got.res.Stdout)
			if detail == "" {
				detail = "no output"
			}
			fmt.Printf("%s✘ BLOCKED%s  (expected ALLOWED — %.1fs) — %s\n", red, reset, elapsed.Seconds(), truncate(detail, 60))
		} else if !t.expected && !got.succeeded {
			passed++
			fmt.Printf("%s✔ BLOCKED%s  (no response — %.1fs, ACL enforced)\n", green, reset, elapsed.Seconds())
		} else {
			failed++
			fmt.Printf("%s✘ ALLOWED%s  (expected BLOCKED — %.1fs) — ACL NOT enforced\n", red, reset, elapsed.Seconds())
		}
	}

	// ── Ping Sweep ───────────────────────────────────────────────────
	header(fmt.Sprintf("Ping Sweep — %d pings from each source → eu-db (%s)", pingCount, bindAddr))
	fmt.Printf("  %s(informational — not counted in pass/fail tally)%s\n", dim, reset)

	type pingSource struct {
		name  string
		note  string // context about what this path tests
		ping  func() runResult
	}

	sources := []pingSource{
		{
			name: "us-euro-viewer",
			note: "Docker 172.21.0.10 → Tailscale mesh → eu-db",
			ping: func() runResult {
				return dockerExecTimeout("us-euro-viewer", fmt.Sprintf("ping -c 1 -W 3 %s", bindAddr), 8*time.Second)
			},
		},
		{
			name: "us-euro-viewer-admin",
			note: "Docker 172.21.0.20 → Tailscale mesh → eu-db",
			ping: func() runResult {
				return dockerExecTimeout("us-euro-viewer-admin", fmt.Sprintf("ping -c 1 -W 3 %s", bindAddr), 8*time.Second)
			},
		},
		{
			name: "us-app-ts",
			note: "Tailscale sidecar 172.21.0.2 → mesh → eu-db",
			ping: func() runResult {
				return dockerExecTimeout("us-app-ts", fmt.Sprintf("ping -c 1 -W 3 %s", bindAddr), 8*time.Second)
			},
		},
		{
			name: "eu-router",
			note: "OrbStack bridge (same L2 as eu-db, no Tailscale)",
			ping: func() runResult {
				return orbRunTimeout("eu-router", fmt.Sprintf("ping -c 1 -W 3 %s", bindAddr), 8*time.Second)
			},
		},
	}

	latencyRe := regexp.MustCompile(`time[=<](\d+\.?\d*)\s*ms`)

	for _, src := range sources {
		fmt.Printf("\n  %s%s[%s]%s  %s%s%s\n", bold, cyan, src.name, reset, dim, src.note, reset)

		var successes int
		var latencies []float64

		for i := 1; i <= pingCount; i++ {
			res := src.ping()
			if res.OK() {
				ms := extractLatency(latencyRe, res.Stdout)
				if ms >= 0 {
					latencies = append(latencies, ms)
				}
				successes++
				fmt.Printf("    ping %2d/%d  %s✔%s", i, pingCount, green, reset)
				if ms >= 0 {
					fmt.Printf("  %.1f ms", ms)
				}
				fmt.Println()
			} else {
				fmt.Printf("    ping %2d/%d  %s✘%s  timeout\n", i, pingCount, red, reset)
			}
		}

		// Summary line
		if successes > 0 && len(latencies) > 0 {
			min, max, avg := stats(latencies)
			fmt.Printf("  %s● %d/%d%s  avg %.1fms  min %.1fms  max %.1fms\n",
				green, successes, pingCount, reset, avg, min, max)
		} else {
			fmt.Printf("  %s● 0/%d%s  all timed out\n", dim, pingCount, reset)
		}
	}

	printSummary(passed, failed)
	return nil
}

// isBlocked returns true if combined stdout+stderr suggests the connection
// was refused, timed out, or otherwise blocked.
func isBlocked(r runResult) bool {
	combined := strings.ToLower(r.Stdout + " " + r.Stderr)
	blocked := []string{"error", "denied", "refused", "timed out", "timeout", "blocked", "can't connect", "no route"}
	for _, kw := range blocked {
		if strings.Contains(combined, kw) {
			return true
		}
	}
	return false
}

// extractLatency pulls the ms value from ping output.
func extractLatency(re *regexp.Regexp, output string) float64 {
	m := re.FindStringSubmatch(output)
	if len(m) < 2 {
		return -1
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return -1
	}
	return v
}

// stats returns min, max, avg of a float slice.
func stats(vals []float64) (min, max, avg float64) {
	min = math.MaxFloat64
	max = -1
	sum := 0.0
	for _, v := range vals {
		sum += v
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	avg = sum / float64(len(vals))
	return
}
