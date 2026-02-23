package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	beadsv1 "github.com/groblegark/kbeads/gen/beads/v1"
	"github.com/spf13/cobra"
)

var commentCmd = &cobra.Command{
	Use:   "comment",
	Short: "Manage bead comments",
}

var commentAddCmd = &cobra.Command{
	Use:   "add <bead-id> <text>...",
	Short: "Add a comment to a bead",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		beadID := args[0]
		text := strings.Join(args[1:], " ")

		resp, err := client.AddComment(context.Background(), &beadsv1.AddCommentRequest{
			BeadId: beadID,
			Author: actor,
			Text:   text,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		c := resp.GetComment()
		if jsonOutput {
			data, err := json.MarshalIndent(c, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(string(data))
		} else {
			fmt.Printf("ID:         %d\n", c.GetId())
			fmt.Printf("Bead:       %s\n", c.GetBeadId())
			fmt.Printf("Author:     %s\n", c.GetAuthor())
			fmt.Printf("Text:       %s\n", c.GetText())
			if c.GetCreatedAt() != nil {
				fmt.Printf("Created At: %s\n", c.GetCreatedAt().AsTime().Format("2006-01-02 15:04:05"))
			}
		}
		return nil
	},
}

var commentListCmd = &cobra.Command{
	Use:   "list <bead-id>",
	Short: "List comments on a bead",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		beadID := args[0]

		resp, err := client.GetComments(context.Background(), &beadsv1.GetCommentsRequest{
			BeadId: beadID,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		comments := resp.GetComments()
		if jsonOutput {
			data, err := json.MarshalIndent(comments, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(string(data))
		} else {
			if len(comments) == 0 {
				fmt.Println("No comments found.")
				return nil
			}
			for i, c := range comments {
				if i > 0 {
					fmt.Println("---")
				}
				createdAt := ""
				if c.GetCreatedAt() != nil {
					createdAt = c.GetCreatedAt().AsTime().Format("2006-01-02 15:04:05")
				}
				fmt.Printf("[%s] %s:\n  %s\n", createdAt, c.GetAuthor(), c.GetText())
			}
		}
		return nil
	},
}

func init() {
	commentCmd.AddCommand(commentAddCmd)
	commentCmd.AddCommand(commentListCmd)
}
