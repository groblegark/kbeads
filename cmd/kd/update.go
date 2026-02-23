package main

import (
	"context"
	"fmt"
	"os"

	beadsv1 "github.com/groblegark/kbeads/gen/beads/v1"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
)

var updateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a bead",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		req := &beadsv1.UpdateBeadRequest{
			Id: id,
		}

		if cmd.Flags().Changed("title") {
			v, _ := cmd.Flags().GetString("title")
			req.Title = proto.String(v)
		}
		if cmd.Flags().Changed("description") {
			v, _ := cmd.Flags().GetString("description")
			req.Description = proto.String(v)
		}
		if cmd.Flags().Changed("status") {
			v, _ := cmd.Flags().GetString("status")
			req.Status = proto.String(v)
		}
		if cmd.Flags().Changed("priority") {
			v, _ := cmd.Flags().GetInt32("priority")
			req.Priority = proto.Int32(v)
		}
		if cmd.Flags().Changed("assignee") {
			v, _ := cmd.Flags().GetString("assignee")
			req.Assignee = proto.String(v)
		}
		if cmd.Flags().Changed("owner") {
			v, _ := cmd.Flags().GetString("owner")
			req.Owner = proto.String(v)
		}
		if cmd.Flags().Changed("notes") {
			v, _ := cmd.Flags().GetString("notes")
			req.Notes = proto.String(v)
		}
		if cmd.Flags().Changed("field") {
			fieldPairs, _ := cmd.Flags().GetStringArray("field")
			fieldsJSON, err := parseFields(fieldPairs)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			req.Fields = fieldsJSON
		}
		resp, err := client.UpdateBead(context.Background(), req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if jsonOutput {
			printBeadJSON(resp.GetBead())
		} else {
			printBeadTable(resp.GetBead())
		}
		return nil
	},
}

func init() {
	updateCmd.Flags().String("title", "", "bead title")
	updateCmd.Flags().StringP("description", "d", "", "bead description")
	updateCmd.Flags().StringP("status", "s", "", "bead status")
	updateCmd.Flags().Int32P("priority", "p", 0, "bead priority")
	updateCmd.Flags().String("assignee", "", "assignee")
	updateCmd.Flags().String("owner", "", "owner")
	updateCmd.Flags().String("notes", "", "notes")
	updateCmd.Flags().StringArrayP("field", "f", nil, "typed field (key=value, repeatable)")
}
