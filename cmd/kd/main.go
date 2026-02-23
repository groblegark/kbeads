package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/groblegark/kbeads/internal/client"
	"github.com/spf13/cobra"
)

var (
	serverAddr string
	httpURL    string
	transport  string
	jsonOutput bool
	actor      string

	beadsClient client.BeadsClient
)

func defaultActor() string {
	out, err := exec.Command("git", "config", "user.name").Output()
	if err == nil {
		name := strings.TrimSpace(string(out))
		if name != "" {
			return name
		}
	}
	return "unknown"
}

func defaultHTTPURL() string {
	if s := os.Getenv("BEADS_HTTP_URL"); s != "" {
		return s
	}
	return "http://localhost:8080"
}

func defaultServer() string {
	if s := os.Getenv("BEADS_SERVER"); s != "" {
		return s
	}
	return "localhost:9090"
}

var rootCmd = &cobra.Command{
	Use:   "kd",
	Short: "CLI client for the Beads service",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		switch transport {
		case "http":
			beadsClient = client.NewHTTPClient(httpURL)
		case "grpc":
			c, err := client.NewGRPCClient(serverAddr)
			if err != nil {
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			beadsClient = c
		default:
			return fmt.Errorf("unknown transport %q (must be http or grpc)", transport)
		}
		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if beadsClient != nil {
			beadsClient.Close()
		}
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&httpURL, "http-url", defaultHTTPURL(), "HTTP server URL")
	rootCmd.PersistentFlags().StringVar(&serverAddr, "server", defaultServer(), "gRPC server address")
	rootCmd.PersistentFlags().StringVar(&transport, "transport", "http", "transport protocol (http or grpc)")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	rootCmd.PersistentFlags().StringVar(&actor, "actor", defaultActor(), "actor name for created_by fields")

	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(showCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(closeCmd)
	rootCmd.AddCommand(reopenCmd)
	rootCmd.AddCommand(doneCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(claimCmd)
	rootCmd.AddCommand(unclaimCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(blockedCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(healthCmd)
	rootCmd.AddCommand(deferCmd)
	rootCmd.AddCommand(undeferCmd)
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(depCmd)
	rootCmd.AddCommand(labelCmd)
	rootCmd.AddCommand(commentCmd)
	rootCmd.AddCommand(graphCmd)
	rootCmd.AddCommand(childrenCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(viewCmd)
	rootCmd.AddCommand(watchCmd)
	rootCmd.AddCommand(contextCmd)
	rootCmd.AddCommand(adviceCmd)
	rootCmd.AddCommand(jackCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
