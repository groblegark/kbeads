package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	beadsv1 "github.com/groblegark/kbeads/gen/beads/v1"
	"github.com/groblegark/kbeads/internal/events"
	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var watchCmd = &cobra.Command{
	Use:   "watch <view-name>",
	Short: "Watch for beads matching a saved view",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		interval, _ := cmd.Flags().GetDuration("interval")
		once, _ := cmd.Flags().GetBool("once")

		// 1. Fetch the view config.
		resp, err := client.GetConfig(context.Background(), &beadsv1.GetConfigRequest{
			Key: "view:" + name,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		var vc viewConfig
		if err := json.Unmarshal(resp.GetConfig().GetValue(), &vc); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing view config: %v\n", err)
			os.Exit(1)
		}

		// 2. Build the ListBeads request.
		req := &beadsv1.ListBeadsRequest{
			Status:   vc.Filter.Status,
			Type:     vc.Filter.Type,
			Kind:     vc.Filter.Kind,
			Labels:   vc.Filter.Labels,
			Assignee: expandVar(vc.Filter.Assignee),
			Search:   vc.Filter.Search,
			Sort:     vc.Sort,
			Limit:    vc.Limit,
		}
		if vc.Filter.Priority != nil {
			req.Priority = wrapperspb.Int32(*vc.Filter.Priority)
		}

		// 3. Setup signal handling.
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()

		seen := make(map[string]time.Time)

		// 4. Initial query.
		if err := queryAndPrint(ctx, req, seen); err != nil {
			return err
		}
		if once {
			return nil
		}

		// 5. Choose event-driven or polling mode.
		natsURL := os.Getenv("BEADS_NATS_URL")
		if natsURL != "" {
			return watchNATS(ctx, natsURL, req, seen)
		}
		return watchPoll(ctx, interval, req, seen)
	},
}

// watchNATS subscribes to NATS events and re-queries on changes with debounce.
func watchNATS(ctx context.Context, natsURL string, req *beadsv1.ListBeadsRequest, seen map[string]time.Time) error {
	// reconnectCh receives a signal when the NATS client reconnects after
	// a disconnect, so we can immediately re-query for missed events.
	reconnectCh := make(chan struct{}, 1)

	sub, err := events.NewNATSSubscriber(natsURL,
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			log.Printf("nats: disconnected: %v", err)
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			log.Printf("nats: reconnected")
			select {
			case reconnectCh <- struct{}{}:
			default:
			}
		}),
	)
	if err != nil {
		return fmt.Errorf("connecting to NATS: %w", err)
	}
	defer sub.Close()

	ch, cancel, err := sub.Subscribe("beads.>")
	if err != nil {
		return fmt.Errorf("subscribing to events: %w", err)
	}
	defer cancel()

	debounce := time.NewTimer(0)
	debounce.Stop()
	// Drain the timer channel in case it fired between NewTimer and Stop.
	select {
	case <-debounce.C:
	default:
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case _, ok := <-ch:
			if !ok {
				return nil
			}
			debounce.Reset(200 * time.Millisecond)
		case <-reconnectCh:
			debounce.Reset(0) // immediate re-query
		case <-debounce.C:
			if err := queryAndPrint(ctx, req, seen); err != nil {
				return err
			}
		}
	}
}

// watchPoll polls for changes at the given interval.
func watchPoll(ctx context.Context, interval time.Duration, req *beadsv1.ListBeadsRequest, seen map[string]time.Time) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(interval):
		}
		if err := queryAndPrint(ctx, req, seen); err != nil {
			return err
		}
	}
}

// queryAndPrint calls ListBeads, diffs against the seen map, and prints any changes.
func queryAndPrint(ctx context.Context, req *beadsv1.ListBeadsRequest, seen map[string]time.Time) error {
	changed, total, err := queryAndDiff(ctx, req, seen)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if len(changed) > 0 {
		if jsonOutput {
			printBeadListJSON(changed)
		} else {
			printBeadListTable(changed, total)
		}
	}
	return nil
}

// queryAndDiff calls ListBeads and returns beads that are new or changed since
// last seen. It updates the seen map in place.
func queryAndDiff(ctx context.Context, req *beadsv1.ListBeadsRequest, seen map[string]time.Time) ([]*beadsv1.Bead, int32, error) {
	listResp, err := client.ListBeads(ctx, req)
	if err != nil {
		return nil, 0, err
	}

	changed := diffBeads(listResp.GetBeads(), seen)
	return changed, listResp.GetTotal(), nil
}

// diffBeads compares beads against the seen map and returns those that are new
// or have a different updated_at timestamp. It updates seen in place.
func diffBeads(beads []*beadsv1.Bead, seen map[string]time.Time) []*beadsv1.Bead {
	var changed []*beadsv1.Bead
	for _, b := range beads {
		var updatedAt time.Time
		if b.GetUpdatedAt() != nil {
			updatedAt = b.GetUpdatedAt().AsTime()
		}
		prev, ok := seen[b.GetId()]
		if !ok || !updatedAt.Equal(prev) {
			changed = append(changed, b)
		}
		seen[b.GetId()] = updatedAt
	}
	return changed
}

func init() {
	watchCmd.Flags().Duration("interval", 5*time.Second, "polling interval")
	watchCmd.Flags().Bool("once", false, "exit after first poll")
}
