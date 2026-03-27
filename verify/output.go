package main

import (
	"fmt"
)

// ANSI color helpers
const (
	green  = "\033[0;32m"
	red    = "\033[0;31m"
	cyan   = "\033[0;36m"
	bold   = "\033[1m"
	dim    = "\033[2m"
	reset  = "\033[0m"
)

func pass(msg string) { fmt.Printf("%s  ✔ PASS%s  %s\n", green, reset, msg) }
func fail(msg string) { fmt.Printf("%s  ✘ FAIL%s  %s\n", red, reset, msg) }
func info(msg string) { fmt.Printf("%s  ℹ INFO%s  %s\n", dim, reset, msg) }
func header(title string) {
	fmt.Printf("\n%s%s── %s ──%s\n", cyan, bold, title, reset)
}
