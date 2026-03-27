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

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
