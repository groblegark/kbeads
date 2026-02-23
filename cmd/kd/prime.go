package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/groblegark/kbeads/internal/client"
	"github.com/groblegark/kbeads/internal/model"
	"github.com/spf13/cobra"
)

var primeCmd = &cobra.Command{
	Use:   "prime",
	Short: "Output session context for an agent (advice injection)",
	Long: `Fetch matching advice beads for the given agent and output them as
a markdown section suitable for injection into agent session context.

This is the kbeads equivalent of "bd prime" advice injection. It fetches all
open advice beads, filters by the agent's subscription labels, groups by
scope, and renders as markdown.

Examples:
  kd prime --agent beads/crew/test-agent
  kd prime --agent beads/crew/test-agent --no-advice
  kd prime --agent beads/crew/test-agent --json`,
	RunE: runPrime,
}

var (
	primeAgent    string
	primeNoAdvice bool
)

func init() {
	primeCmd.Flags().StringVar(&primeAgent, "agent", "", "agent ID for subscription matching (required)")
	primeCmd.Flags().BoolVar(&primeNoAdvice, "no-advice", false, "suppress advice output")
	primeCmd.MarkFlagRequired("agent")
}

func runPrime(cmd *cobra.Command, args []string) error {
	if primeNoAdvice {
		return nil
	}

	outputAdvice(os.Stdout, beadsClient, primeAgent)
	return nil
}

// outputAdvice fetches open advice beads, filters by agent subscriptions,
// groups by scope, and writes markdown to w.
func outputAdvice(w io.Writer, c client.BeadsClient, agentID string) {
	ctx := context.Background()

	// Fetch all open advice beads.
	resp, err := c.ListBeads(ctx, &client.ListBeadsRequest{
		Type:   []string{"advice"},
		Status: []string{"open"},
		Limit:  500,
	})
	if err != nil {
		return // fail silently — don't block agent startup
	}

	if len(resp.Beads) == 0 {
		return
	}

	// Build agent subscriptions.
	subs := model.BuildAgentSubscriptions(agentID, nil)

	// Filter by subscription matching.
	type matchedAdvice struct {
		Bead          *model.Bead
		MatchedLabels []string
	}
	var matched []matchedAdvice
	for _, bead := range resp.Beads {
		if model.MatchesSubscriptions(bead.Labels, subs) {
			ml := findMatchedAdviceLabels(bead.Labels, subs)
			matched = append(matched, matchedAdvice{Bead: bead, MatchedLabels: ml})
		}
	}

	if len(matched) == 0 {
		return
	}

	// JSON mode.
	if jsonOutput {
		type jsonItem struct {
			ID            string   `json:"id"`
			Title         string   `json:"title"`
			Description   string   `json:"description,omitempty"`
			Labels        []string `json:"labels"`
			MatchedLabels []string `json:"matched_labels"`
		}
		items := make([]jsonItem, len(matched))
		for i, m := range matched {
			items[i] = jsonItem{
				ID:            m.Bead.ID,
				Title:         m.Bead.Title,
				Description:   m.Bead.Description,
				Labels:        m.Bead.Labels,
				MatchedLabels: m.MatchedLabels,
			}
		}
		data, _ := json.MarshalIndent(items, "", "  ")
		fmt.Fprintln(w, string(data))
		return
	}

	// Group by scope and render.
	type scopeGroup struct {
		Scope  string
		Target string
		Header string
		Items  []matchedAdvice
	}

	groupMap := make(map[string]*scopeGroup)
	for _, m := range matched {
		scope, target := categorizeScope(m.Bead.Labels)
		key := scope + ":" + target
		g, ok := groupMap[key]
		if !ok {
			g = &scopeGroup{
				Scope:  scope,
				Target: target,
				Header: buildHeader(scope, target),
			}
			groupMap[key] = g
		}
		g.Items = append(g.Items, m)
	}

	var groups []*scopeGroup
	for _, g := range groupMap {
		groups = append(groups, g)
	}
	sort.Slice(groups, func(i, j int) bool {
		return groupSortKey(groups[i].Scope, groups[i].Target) < groupSortKey(groups[j].Scope, groups[j].Target)
	})

	fmt.Fprintf(w, "\n## Advice (%d items)\n\n", len(matched))
	for _, g := range groups {
		for _, item := range g.Items {
			fmt.Fprintf(w, "**[%s]** %s\n", g.Header, item.Bead.Title)
			desc := item.Bead.Description
			if desc != "" && desc != item.Bead.Title {
				for _, line := range strings.Split(desc, "\n") {
					fmt.Fprintf(w, "  %s\n", line)
				}
			}
			fmt.Fprintln(w)
		}
	}
}

// findMatchedAdviceLabels returns the subset of advice labels that matched
// the agent's subscriptions.
func findMatchedAdviceLabels(adviceLabels, subscriptions []string) []string {
	subSet := make(map[string]bool, len(subscriptions))
	for _, s := range subscriptions {
		subSet[s] = true
	}
	seen := make(map[string]bool)
	var matched []string
	for _, l := range adviceLabels {
		clean := model.StripGroupPrefix(l)
		if subSet[clean] && !seen[clean] {
			matched = append(matched, clean)
			seen[clean] = true
		}
	}
	return matched
}

// categorizeScope determines the most specific scope from advice labels.
func categorizeScope(labels []string) (scope, target string) {
	for _, l := range labels {
		clean := model.StripGroupPrefix(l)
		switch {
		case strings.HasPrefix(clean, "agent:"):
			return "agent", strings.TrimPrefix(clean, "agent:")
		case strings.HasPrefix(clean, "role:"):
			scope, target = "role", strings.TrimPrefix(clean, "role:")
		case strings.HasPrefix(clean, "rig:") && scope != "role":
			scope, target = "rig", strings.TrimPrefix(clean, "rig:")
		case clean == "global" && scope == "":
			scope, target = "global", ""
		}
	}
	if scope == "" {
		scope = "global"
	}
	return scope, target
}

func buildHeader(scope, target string) string {
	switch scope {
	case "global":
		return "Global"
	case "rig":
		return "Rig: " + target
	case "role":
		return "Role: " + target
	case "agent":
		return "Agent: " + target
	default:
		return scope
	}
}

func groupSortKey(scope, target string) string {
	switch scope {
	case "global":
		return "0:" + target
	case "rig":
		return "1:" + target
	case "role":
		return "2:" + target
	case "agent":
		return "3:" + target
	default:
		return "9:" + target
	}
}
