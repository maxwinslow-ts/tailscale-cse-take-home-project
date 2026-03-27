// transit_encrypt.go — Packet-capture comparison proving Tailscale encrypts
// database traffic. Captures plaintext MySQL on the insecure Docker bridge
// (us-app), then captures WireGuard UDP on eu-router's physical interface.
package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func transitEncryptCmd() *cobra.Command {
	var captureSecs int

	cmd := &cobra.Command{
		Use:   "transit-encrypt",
		Short: "Prove Tailscale encrypts database traffic in transit vs plaintext insecure path",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTransitEncrypt(captureSecs)
		},
	}

	cmd.Flags().IntVarP(&captureSecs, "seconds", "s", 15, "Seconds to run each tcpdump capture")
	return cmd
}

// plaintextKeywords are MySQL protocol strings we expect to find in unencrypted traffic.
var plaintextKeywords = []string{
	"SELECT", "select", "famous_americans", "famous_europeans",
	"INSERT", "FROM", "WHERE", "mysql_native_password",
}

// containsPlaintext checks whether any MySQL-related plaintext is present.
func containsPlaintext(output string) []string {
	var found []string
	seen := map[string]bool{}
	for _, kw := range plaintextKeywords {
		if !seen[kw] && strings.Contains(output, kw) {
			found = append(found, kw)
			seen[kw] = true
		}
	}
	return found
}

// tcpdumpHasPackets checks whether tcpdump output contains actual captured packet
// lines (e.g. "12:34:56.789 IP ...") rather than relying on the summary line which
// is unreliable when tcpdump is killed by SIGTERM via timeout.
var tcpdumpPacketLine = regexp.MustCompile(`\d{2}:\d{2}:\d{2}\.\d+ IP `)

func tcpdumpHasPackets(output string) bool {
	return tcpdumpPacketLine.MatchString(output)
}

// extractReadableSnippets pulls lines from tcpdump -A output that contain readable ASCII.
// Lines containing any keyword are prioritized and shown first, ensuring the most
// interesting plaintext evidence (SQL queries, table names, credentials) appears at the
// top of the snippet output even if the TCP handshake lines come first in the capture.
func extractReadableSnippets(output string, max int, keywords []string) []string {
	var keywordLines []string // lines with plaintext keywords — shown first
	var otherLines []string   // other readable lines — fill remaining slots

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Skip tcpdump header/metadata lines
		if strings.HasPrefix(line, "tcpdump:") || strings.HasPrefix(line, "listening on") ||
			strings.Contains(line, " packets captured") || strings.Contains(line, " packets received") ||
			strings.Contains(line, " packets dropped") {
			continue
		}

		hasKeyword := false
		for _, kw := range keywords {
			if strings.Contains(line, kw) {
				hasKeyword = true
				break
			}
		}

		if hasKeyword {
			keywordLines = append(keywordLines, line)
		} else {
			// Fall back to printable-ratio heuristic
			printable := 0
			for _, c := range line {
				if c >= 0x20 && c <= 0x7e {
					printable++
				}
			}
			if len(line) > 0 && float64(printable)/float64(len(line)) > 0.6 {
				otherLines = append(otherLines, line)
			}
		}
	}

	// Keyword lines first, then fill remaining slots with context
	var snippets []string
	for _, l := range keywordLines {
		if len(snippets) >= max {
			break
		}
		snippets = append(snippets, l)
	}
	for _, l := range otherLines {
		if len(snippets) >= max {
			break
		}
		snippets = append(snippets, l)
	}
	return snippets
}

func runTransitEncrypt(captureSecs int) error {
	passed, failed := 0, 0
	p := func(msg string) { passed++; pass(msg) }
	f := func(msg string) { failed++; fail(msg) }

	captureTimeout := time.Duration(captureSecs) * time.Second

	// ── Install tcpdump if needed ────────────────────────────────────
	header("Setup — installing tcpdump on us-app + eu-router")
	fmt.Printf("  %sInstalling tcpdump on VMs if needed…%s\n", dim, reset)
	orbRunTimeout("us-app", "sudo apt-get install -y tcpdump >/dev/null 2>&1", 30*time.Second)
	orbRunTimeout("eu-router", "sudo apt-get install -y tcpdump >/dev/null 2>&1", 30*time.Second)
	info("tcpdump ready")

	// ── Discover insecure Docker bridge interface on us-app ──────────
	header("Interface Discovery (us-app + eu-router)")

	brRes := orbRun("us-app", `ip -o addr show | grep '172.22.0' | awk '{print $2}' | head -1`)
	insecureBridge := strings.TrimSpace(brRes.Output())
	if insecureBridge == "" {
		f("Could not find Docker bridge for insecure 172.22.0.0/24 network on us-app")
		printSummary(passed, failed)
		return nil
	}
	info(fmt.Sprintf("Insecure Docker bridge interface: %s", insecureBridge))

	// ── Discover eu-router physical interface ────────────────────────
	phyRes := orbRun("eu-router", `ip route show default | awk '{print $5}' | head -1`)
	physIface := strings.TrimSpace(phyRes.Output())
	if physIface == "" {
		f("Could not find physical interface on eu-router")
		printSummary(passed, failed)
		return nil
	}
	info(fmt.Sprintf("eu-router physical interface: %s", physIface))

	// ── Discover eu-db MySQL bind address (for context output) ───────
	bindAddr := discoverBindAddr()
	if bindAddr != "" {
		info(fmt.Sprintf("eu-db MySQL bind address: %s", bindAddr))
	}

	// ══════════════════════════════════════════════════════════════════
	// INSECURE PATH — plaintext MySQL on Docker bridge
	// ══════════════════════════════════════════════════════════════════
	header("INSECURE PATH — insecure-demo-viewer ↔ insecure-demo-db (no Tailscale)")
	fmt.Printf("  %sCapture point: us-app VM, interface %s (Docker bridge 172.22.0.0/24)%s\n", dim, insecureBridge, reset)
	fmt.Printf("  %sExpect: plaintext MySQL queries and data visible in packet payloads%s\n", dim, reset)
	fmt.Println()

	insecureCmd := fmt.Sprintf(
		"sudo tcpdump -i %s -A -nn 'port 3306' 2>&1 & PID=$!; sleep %d; sudo kill -INT $PID 2>/dev/null; wait $PID 2>/dev/null; true",
		insecureBridge, captureSecs,
	)
	fmt.Printf("  %sCapturing for %ds on %s:3306…%s\n", dim, captureSecs, insecureBridge, reset)
	insecureRes := orbRunTimeout("us-app", insecureCmd, captureTimeout+10*time.Second)

	insecureOutput := insecureRes.Stdout + insecureRes.Stderr
	insecureMatches := containsPlaintext(insecureOutput)

	if len(insecureMatches) > 0 {
		p(fmt.Sprintf("Plaintext MySQL content detected in insecure traffic! Found: %s",
			strings.Join(insecureMatches, ", ")))
	} else if !tcpdumpHasPackets(insecureOutput) {
		f("No traffic captured on insecure bridge — is insecure-demo running?")
	} else {
		f("Traffic captured but no expected MySQL plaintext found")
	}

	// Show captured snippets
	fmt.Println()
	fmt.Printf("  %s%s── Captured payload snippets (insecure path) ──%s\n", cyan, bold, reset)
	snippets := extractReadableSnippets(insecureOutput, 20, plaintextKeywords)
	if len(snippets) > 0 {
		for _, s := range snippets {
			// Highlight known plaintext keywords in red to draw attention
			display := s
			for _, kw := range plaintextKeywords {
				if strings.Contains(display, kw) {
					display = strings.ReplaceAll(display, kw, red+bold+kw+reset)
				}
			}
			fmt.Printf("    %s\n", truncate(display, 120))
		}
	} else {
		fmt.Printf("    %s(no readable payload captured)%s\n", dim, reset)
	}

	// ══════════════════════════════════════════════════════════════════
	// SECURE PATH — WireGuard encrypted UDP on eu-router physical iface
	// ══════════════════════════════════════════════════════════════════
	fmt.Println()
	header("SECURE PATH — WireGuard tunnel arriving at eu-router (from us-app-ts)")
	fmt.Printf("  %sCapture point: eu-router VM, interface %s (physical/upstream)%s\n", dim, physIface, reset)
	fmt.Printf("  %sThis is where Tailscale WireGuard packets arrive — still encrypted.%s\n", dim, reset)
	fmt.Printf("  %sExpect: encrypted UDP payloads with NO readable MySQL content%s\n", dim, reset)
	fmt.Println()

	// Capture WireGuard traffic: try port 41641, fall back to broader UDP.
	// Use SIGINT for clean shutdown.
	secureCmd := fmt.Sprintf(
		"sudo tcpdump -i %s -A -nn 'udp port 41641' 2>&1 & PID=$!; sleep %d; sudo kill -INT $PID 2>/dev/null; wait $PID 2>/dev/null; true",
		physIface, captureSecs,
	)
	fmt.Printf("  %sCapturing for %ds on %s (WireGuard UDP 41641)…%s\n", dim, captureSecs, physIface, reset)
	secureRes := orbRunTimeout("eu-router", secureCmd, captureTimeout+10*time.Second)

	secureOutput := secureRes.Stdout + secureRes.Stderr

	// If no actual packet lines on 41641, try broader UDP filter
	if !tcpdumpHasPackets(secureOutput) {
		info("No traffic on UDP 41641 — trying broader UDP filter (excluding DNS/mDNS)…")
		secureCmd = fmt.Sprintf(
			"sudo tcpdump -i %s -A -nn 'udp and not port 53 and not port 5353' 2>&1 & PID=$!; sleep %d; sudo kill -INT $PID 2>/dev/null; wait $PID 2>/dev/null; true",
			physIface, captureSecs,
		)
		secureRes = orbRunTimeout("eu-router", secureCmd, captureTimeout+10*time.Second)
		secureOutput = secureRes.Stdout + secureRes.Stderr
	}

	secureMatches := containsPlaintext(secureOutput)
	hasSecurePackets := tcpdumpHasPackets(secureOutput)

	if !hasSecurePackets {
		f("No WireGuard traffic captured — is the euro-viewer app running and querying eu-db?")
	} else if len(secureMatches) == 0 {
		p("WireGuard packets captured — NO plaintext MySQL content visible (encrypted!)")
	} else {
		f(fmt.Sprintf("WARNING: Plaintext MySQL content found in WireGuard tunnel! Found: %s",
			strings.Join(secureMatches, ", ")))
	}

	// Show captured snippets
	fmt.Println()
	fmt.Printf("  %s%s── Captured payload snippets (secure path) ──%s\n", cyan, bold, reset)
	secSnippets := extractReadableSnippets(secureOutput, 20, plaintextKeywords)
	if len(secSnippets) > 0 {
		for _, s := range secSnippets {
			display := s
			// If somehow plaintext leaks, highlight it in red
			for _, kw := range plaintextKeywords {
				if strings.Contains(display, kw) {
					display = strings.ReplaceAll(display, kw, red+bold+kw+reset)
				}
			}
			fmt.Printf("    %s\n", truncate(display, 120))
		}
	} else {
		fmt.Printf("    %s(no readable payload captured)%s\n", dim, reset)
	}

	// ══════════════════════════════════════════════════════════════════
	// Comparison Summary
	// ══════════════════════════════════════════════════════════════════
	fmt.Println()
	header("Comparison")
	fmt.Printf("  %-40s  %s\n",
		fmt.Sprintf("%s%sInsecure (no Tailscale)%s", red, bold, reset),
		fmt.Sprintf("%s%sSecure (Tailscale WireGuard)%s", green, bold, reset))
	fmt.Printf("  %-40s  %s\n",
		fmt.Sprintf("Plaintext keywords found: %s%d%s", red, len(insecureMatches), reset),
		fmt.Sprintf("Plaintext keywords found: %s%d%s", green, len(secureMatches), reset))

	if len(insecureMatches) > 0 {
		fmt.Printf("  %s⚠  SQL queries, table names, and data%s    ", red, reset)
	} else {
		fmt.Printf("  %-40s  ", "(no capture)")
	}
	if len(secureMatches) == 0 && hasSecurePackets {
		fmt.Printf("%s✔  Opaque encrypted UDP payloads%s", green, reset)
	} else if len(secureMatches) > 0 {
		fmt.Printf("%s✘  Plaintext leak detected!%s", red, reset)
	}
	fmt.Println()
	fmt.Printf("  %s     visible in packet capture%s            %s     only WireGuard ciphertext visible%s\n", red, reset, green, reset)

	// ── Summary ──────────────────────────────────────────────────────
	printSummary(passed, failed)
	return nil
}
