package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	beadsv1 "github.com/groblegark/kbeads/gen/beads/v1"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configs (type, view, context definitions)",
}

var configCreateCmd = &cobra.Command{
	Use:     "create <key> <json-value>",
	Aliases: []string{"new"},
	Short:   "Create or update a config",
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := []byte(args[1])

		// Validate that value is valid JSON.
		if !json.Valid(value) {
			fmt.Fprintln(os.Stderr, "Error: value must be valid JSON")
			os.Exit(1)
		}

		resp, err := client.SetConfig(context.Background(), &beadsv1.SetConfigRequest{
			Key:   key,
			Value: value,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		printConfigJSON(resp.GetConfig())
		return nil
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a config by key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := client.GetConfig(context.Background(), &beadsv1.GetConfigRequest{
			Key: args[0],
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		printConfigJSON(resp.GetConfig())
		return nil
	},
}

var configListCmd = &cobra.Command{
	Use:   "list [namespace]",
	Short: "List configs by namespace (e.g. view, type, context)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		namespace := ""
		if len(args) > 0 {
			namespace = args[0]
		}
		if namespace == "" {
			fmt.Fprintln(os.Stderr, "Error: namespace argument is required")
			os.Exit(1)
		}

		resp, err := client.ListConfigs(context.Background(), &beadsv1.ListConfigsRequest{
			Namespace: namespace,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		for _, c := range resp.GetConfigs() {
			printConfigJSON(c)
		}
		if len(resp.GetConfigs()) == 0 {
			fmt.Println("No configs found.")
		}
		return nil
	},
}

var configDeleteCmd = &cobra.Command{
	Use:   "delete <key>",
	Short: "Delete a config by key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := client.DeleteConfig(context.Background(), &beadsv1.DeleteConfigRequest{
			Key: args[0],
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Deleted config %q\n", args[0])
		return nil
	},
}

func printConfigJSON(c *beadsv1.Config) {
	// Pretty-print by unmarshalling the value bytes so they render as JSON, not base64.
	var valueObj any
	_ = json.Unmarshal(c.GetValue(), &valueObj)

	out := map[string]any{
		"key":   c.GetKey(),
		"value": valueObj,
	}
	if c.GetCreatedAt() != nil {
		out["created_at"] = c.GetCreatedAt().AsTime().Format("2006-01-02T15:04:05Z")
	}
	if c.GetUpdatedAt() != nil {
		out["updated_at"] = c.GetUpdatedAt().AsTime().Format("2006-01-02T15:04:05Z")
	}

	data, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(data))
}

func init() {
	configCmd.AddCommand(configCreateCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configDeleteCmd)
}
