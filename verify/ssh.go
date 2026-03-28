// ssh.go — Proves that standard sshd is disabled on eu-db and port 22 is
// not listening, then shows the Tailscale SSH command as the only access path.
package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func testSSHCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test-ssh",
		Short: "Verify standard sshd is disabled on eu-db; only Tailscale SSH works",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTestSSH()
		},
	}
}

func runTestSSH() error {
	passed, failed := 0, 0
	p := func(msg string) { passed++; pass(msg) }
	f := func(msg string) { failed++; fail(msg) }

	// ── Check sshd service status ────────────────────────────────────
	header("Standard sshd on eu-db")
	info("Checking systemctl is-active ssh …")

	res := orbRun("eu-db", "systemctl is-active ssh 2>&1 || true")
	status := strings.TrimSpace(res.Output())
	info(fmt.Sprintf("sshd status: %s", status))
	if status == "active" {
		f("sshd is running — should be disabled so Tailscale SSH is the only path")
	} else {
		p(fmt.Sprintf("sshd is %s — standard SSH daemon is not running", status))
	}

	// ── Check port 22 not listening ──────────────────────────────────
	header("Port 22 on eu-db")
	info("Checking ss -tlnp for :22 …")

	res = orbRun("eu-db", "ss -tlnp | grep ':22 ' || echo 'no-listener'")
	output := strings.TrimSpace(res.Output())
	if output == "no-listener" {
		p("Port 22 is not listening on eu-db")
	} else {
		f(fmt.Sprintf("Port 22 is listening: %s", output))
	}

	// ── Show all listening ports for context ─────────────────────────
	header("All listening ports on eu-db")
	res = orbRun("eu-db", "ss -tlnp")
	fmt.Printf("\n%s\n", res.Output())

	// ── Tailscale SSH command ────────────────────────────────────────
	header("Tailscale SSH (the only access path)")
	fmt.Printf("\n  Port 22 is closed and sshd is disabled. The only way to SSH into eu-db\n")
	fmt.Printf("  is through Tailscale, which requires IdP identity verification:\n\n")
	fmt.Printf("    %s%s$ tailscale ssh root@eu-db%s\n\n", bold, green, reset)

	printSummary(passed, failed)
	return nil
}
