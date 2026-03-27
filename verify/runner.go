// runner.go — Command execution helpers. Three execution contexts:
//
//	orbRun(vm, cmd)              run a shell command on an OrbStack VM
//	dockerExec(container, cmd)   run inside a Docker container on the us-app VM
//	localRun(cmd)                run on the local Mac host
//
// Also provides discoverBindAddr() which reads MySQL's bind-address from eu-db.
package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const defaultTimeout = 10 * time.Second

// runResult holds the output and error from a command execution.
type runResult struct {
	Stdout string
	Stderr string
	Err    error
}

func (r runResult) OK() bool    { return r.Err == nil }
func (r runResult) Output() string { return strings.TrimSpace(r.Stdout) }

// orbRun executes a command on an OrbStack VM.
func orbRun(vm string, cmd string) runResult {
	return orbRunTimeout(vm, cmd, defaultTimeout)
}

func orbRunTimeout(vm string, cmd string, timeout time.Duration) runResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	c := exec.CommandContext(ctx, "orb", "run", "-m", vm, "bash", "-c", cmd)
	var stdout, stderr strings.Builder
	c.Stdout = &stdout
	c.Stderr = &stderr
	err := c.Run()
	return runResult{Stdout: stdout.String(), Stderr: stderr.String(), Err: err}
}

// dockerExec executes a command in a Docker container on us-app.
func dockerExec(container string, cmd string) runResult {
	return dockerExecTimeout(container, cmd, defaultTimeout)
}

func dockerExecTimeout(container string, cmd string, timeout time.Duration) runResult {
	wrapped := fmt.Sprintf("sudo docker exec %s sh -c %s", container, shellQuote(cmd))
	return orbRunTimeout("us-app", wrapped, timeout)
}

// localRun executes a command on the local Mac host.
func localRun(cmd string) runResult {
	return localRunTimeout(cmd, defaultTimeout)
}

func localRunTimeout(cmd string, timeout time.Duration) runResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	c := exec.CommandContext(ctx, "bash", "-c", cmd)
	var stdout, stderr strings.Builder
	c.Stdout = &stdout
	c.Stderr = &stderr
	err := c.Run()
	return runResult{Stdout: stdout.String(), Stderr: stderr.String(), Err: err}
}

// shellQuote wraps a string in single quotes, escaping internal single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// discoverBindAddr reads MySQL's bind-address from eu-db's mysqld config.
func discoverBindAddr() string {
	res := orbRun("eu-db", `grep "^bind-address" /etc/mysql/mysql.conf.d/mysqld.cnf`)
	if res.OK() {
		parts := strings.Fields(res.Output())
		if len(parts) >= 3 {
			return parts[len(parts)-1]
		}
	}
	return ""
}
