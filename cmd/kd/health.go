package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check the health of the beads service",
	RunE: func(cmd *cobra.Command, args []string) error {
		status, err := beadsClient.Health(context.Background())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if jsonOutput {
			out := map[string]string{"status": status}
			data, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(string(data))
		} else {
			fmt.Printf("Health: %s\n", status)
		}

		if status != "ok" {
			os.Exit(1)
		}
		return nil
	},
}
