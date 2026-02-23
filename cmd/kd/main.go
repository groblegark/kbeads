package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	beadsv1 "github.com/groblegark/kbeads/gen/beads/v1"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	serverAddr string
	jsonOutput bool
	actor      string

	conn   *grpc.ClientConn
	client beadsv1.BeadsServiceClient
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
		var err error
		conn, err = grpc.NewClient(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return fmt.Errorf("failed to connect to server: %w", err)
		}
		client = beadsv1.NewBeadsServiceClient(conn)
		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if conn != nil {
			conn.Close()
		}
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&serverAddr, "server", defaultServer(), "gRPC server address")
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
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
