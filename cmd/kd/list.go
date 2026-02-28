package main

import (
	"context"
	"fmt"
	"os"

	"github.com/groblegark/kbeads/internal/client"
	"github.com/groblegark/kbeads/internal/model"
	"github.com/spf13/cobra"
)

var (
	listProjectFlag     string
	listAllProjectsFlag bool
	listAllTypesFlag    bool
)

// noiseTypes are bead types excluded from kd list by default.
// Use --all-types to include them.
var noiseTypes = map[string]bool{
	"advice": true, "agent": true, "artifact": true, "config": true,
	"decision": true, "formula": true, "gate": true, "message": true,
	"molecule": true, "runbook": true,
}

var listCmd = &cobra.Command{
	Use:     "list",
	Short:   "List beads",
	GroupID: "beads",
	RunE: func(cmd *cobra.Command, args []string) error {
		status, _ := cmd.Flags().GetStringSlice("status")
		beadType, _ := cmd.Flags().GetStringSlice("type")
		kind, _ := cmd.Flags().GetStringSlice("kind")
		limit, _ := cmd.Flags().GetInt("limit")
		assignee, _ := cmd.Flags().GetString("assignee")
		offset, _ := cmd.Flags().GetInt("offset")
		fieldFlags, _ := cmd.Flags().GetStringArray("field")
		noBlockers, _ := cmd.Flags().GetBool("no-blockers")
		sort, _ := cmd.Flags().GetString("sort")

		// When no explicit --type filter is given and --all-types is not set,
		// apply client-side noise filtering after fetching.
		filterNoise := !listAllTypesFlag && len(beadType) == 0

		req := &client.ListBeadsRequest{
			Status:     status,
			Type:       beadType,
			Kind:       kind,
			Limit:      limit,
			Assignee:   assignee,
			Offset:     offset,
			NoOpenDeps: noBlockers,
			Sort:       sort,
		}

		if !listAllProjectsFlag && listProjectFlag != "" {
			req.Labels = append(req.Labels, "project:"+listProjectFlag)
		}

		if len(fieldFlags) > 0 {
			req.FieldFilters = make(map[string]string, len(fieldFlags))
			for _, f := range fieldFlags {
				k, v, ok := splitField(f)
				if !ok {
					fmt.Fprintf(os.Stderr, "Error: invalid field filter %q (expected key=value)\n", f)
					os.Exit(1)
				}
				req.FieldFilters[k] = v
			}
		}

		resp, err := beadsClient.ListBeads(context.Background(), req)
		if err != nil {
			return fmt.Errorf("listing beads: %w", err)
		}

		beads := resp.Beads
		if filterNoise {
			beads = filterOutNoiseBeads(beads)
		}

		if jsonOutput {
			printBeadListJSON(beads)
		} else {
			printBeadListTable(beads, resp.Total)
		}
		return nil
	},
}

func init() {
	listCmd.Flags().StringSliceP("status", "s", nil, "filter by status (repeatable)")
	listCmd.Flags().StringSliceP("type", "t", nil, "filter by type (repeatable)")
	listCmd.Flags().StringSliceP("kind", "k", nil, "filter by kind (repeatable)")
	listCmd.Flags().Int("limit", 20, "maximum number of beads to return")
	listCmd.Flags().String("assignee", "", "filter by assignee")
	listCmd.Flags().Int("offset", 0, "offset for pagination")
	listCmd.Flags().StringArrayP("field", "f", nil, "filter by custom field (key=value, repeatable)")
	listCmd.Flags().Bool("no-blockers", false, "only show beads with no open/in_progress/deferred dependencies")
	listCmd.Flags().String("sort", "", "sort column: priority, created_at, updated_at, title, status, type (prefix with - for descending, e.g. -priority)")
	listCmd.Flags().StringVar(&listProjectFlag, "project", resolveProject(), "filter by project label (default: $KD_PROJECT or $BOAT_PROJECT)")
	listCmd.Flags().BoolVar(&listAllProjectsFlag, "all-projects", false, "show beads from all projects (disables project filter)")
	listCmd.Flags().BoolVar(&listAllTypesFlag, "all-types", false, "include infrastructure bead types (advice, agent, artifact, config, decision, etc.)")
}

// filterOutNoiseBeads removes infrastructure bead types from a list.
func filterOutNoiseBeads(beads []*model.Bead) []*model.Bead {
	var filtered []*model.Bead
	for _, b := range beads {
		if noiseTypes[string(b.Type)] {
			continue
		}
		filtered = append(filtered, b)
	}
	return filtered
}
