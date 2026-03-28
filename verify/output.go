// output.go — Colored terminal output: pass/fail badges, info lines, section
// headers, and the shared pass/fail summary printed at the end of each command.
package main

import (
	"fmt"
	"strings"
)

// ANSI color codes used throughout the CLI output.
const (
	green = "\033[0;32m"
	red   = "\033[0;31m"
	cyan  = "\033[0;36m"
	bold  = "\033[1m"
	dim   = "\033[2m"
	reset = "\033[0m"
)

func pass(msg string) { fmt.Printf("%s  ✔ PASS%s  %s\n", green, reset, msg) }
func fail(msg string) { fmt.Printf("%s  ✘ FAIL%s  %s\n", red, reset, msg) }
func info(msg string) { fmt.Printf("%s  ℹ INFO%s  %s\n", dim, reset, msg) }
func header(title string) {
	fmt.Printf("\n%s%s── %s ──%s\n", cyan, bold, title, reset)
}

// printSummary outputs the final pass/fail tally for a command.
func printSummary(passed, failed int) {
	fmt.Printf("\n%s════════════════════════════════════════%s\n", bold, reset)
	fmt.Printf("%s  Passed: %d%s  %s  Failed: %d%s\n", green, passed, reset, red, failed, reset)
	if failed == 0 {
		fmt.Printf("%s%s  All checks passed! ✅%s\n", green, bold, reset)
	} else {
		fmt.Printf("%s%s  Some checks failed. Review output above.%s\n", red, bold, reset)
	}
	fmt.Printf("%s════════════════════════════════════════%s\n", bold, reset)
}

// truncate shortens s to n characters, replacing newlines with spaces.
func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
