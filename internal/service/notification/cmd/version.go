package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newVersionCmd creates the version command
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Notification Service Version: %s\n", version)
		},
	}
}

