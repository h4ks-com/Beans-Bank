package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "beapin",
	Short: "Bean Bank - Bean currency management system",
	Long: `Bean Bank is a currency management system for h4ks.com.

It provides a REST API for managing bean transactions, wallets, and harvests.

Run 'beapin serve' to start the server, or 'beapin import' to import wallets.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(importCmd)
}
