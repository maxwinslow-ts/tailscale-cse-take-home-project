// verify — CLI tool that validates the Sovereign Mesh security posture.
//
// Subcommands:
//
//	mysql-access     probe eu-db:3306 from every network vantage point to confirm ACLs
//	source-ip        prove SNAT is disabled (containers keep distinct source IPs)
//	transit-encrypt  packet-capture comparison of insecure vs. WireGuard paths
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "verify",
		Short: "Security posture and connectivity verification for Sovereign Mesh",
	}

	root.AddCommand(mysqlCmd())
	root.AddCommand(sourceIPCmd())
	root.AddCommand(transitEncryptCmd())
	root.AddCommand(testSSHCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
