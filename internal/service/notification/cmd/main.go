package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const (
	version     = "1.0.0"
	defaultPort = "8082"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// newRootCmd creates and configures the root command
func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "notification-service",
		Short: "Notification service",
		Long:  `Standalone notification service with PostgreSQL LISTEN/NOTIFY, Redis Streams, and multiple sender support.`,
	}

	rootCmd.AddCommand(newServeCmd())
	rootCmd.AddCommand(newMigrateCmd())
	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newAPICmd())

	return rootCmd
}
