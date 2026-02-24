// Package nudge delivers real-time notifications to agents via their PTY sessions.
package nudge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Nudger sends a real-time notification to an agent by name.
type Nudger interface {
	Nudge(ctx context.Context, agentName, message string) error
}

// NoopNudger silently discards all nudges. Used when no mux is configured.
type NoopNudger struct{}

func (NoopNudger) Nudge(_ context.Context, _, _ string) error { return nil }

// CoopMuxNudger delivers nudges by injecting text into an agent's PTY via the
// coop mux session input API. It finds the target session by matching the agent
// name against the pod name in session metadata.
//
// API used:
//
//	GET  {muxURL}/api/v1/sessions              → list sessions + metadata
//	POST {muxURL}/api/v1/sessions/{id}/input   → inject text into PTY
type CoopMuxNudger struct {
	muxURL     string
	muxToken   string
	httpClient *http.Client
}

// NewCoopMuxNudger creates a CoopMuxNudger using the given mux URL and bearer token.
func NewCoopMuxNudger(muxURL, muxToken string) *CoopMuxNudger {
	return &CoopMuxNudger{
		muxURL:   strings.TrimRight(muxURL, "/"),
		muxToken: muxToken,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// muxSession is the relevant subset of a coop mux session record.
type muxSession struct {
	ID       string `json:"id"`
	Metadata struct {
		K8s struct {
			Pod string `json:"pod"`
		} `json:"k8s"`
	} `json:"metadata"`
}

// Nudge finds the active session for agentName and injects a system-reminder
// into the agent's PTY. If the agent is offline the nudge is silently skipped.
func (n *CoopMuxNudger) Nudge(ctx context.Context, agentName, message string) error {
	sessionID, err := n.findSession(ctx, agentName)
	if err != nil {
		return fmt.Errorf("nudge: find session: %w", err)
	}
	if sessionID == "" {
		slog.Debug("nudge: no active session", "agent", agentName)
		return nil
	}

	text := fmt.Sprintf("<system-reminder>\n%s\n</system-reminder>\n", message)
	return n.injectInput(ctx, sessionID, text)
}

// findSession queries the mux for a session whose pod name contains agentName.
func (n *CoopMuxNudger) findSession(ctx context.Context, agentName string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, n.muxURL+"/api/v1/sessions", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+n.muxToken)

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("mux sessions: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var sessions []muxSession
	if err := json.Unmarshal(body, &sessions); err != nil {
		return "", fmt.Errorf("parse sessions: %w", err)
	}

	for _, s := range sessions {
		if strings.Contains(s.Metadata.K8s.Pod, agentName) {
			return s.ID, nil
		}
	}
	return "", nil
}

// injectInput POSTs text into the session's PTY via the mux input endpoint.
func (n *CoopMuxNudger) injectInput(ctx context.Context, sessionID, text string) error {
	payload, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/v1/sessions/%s/input", n.muxURL, sessionID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+n.muxToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("mux input: HTTP %d", resp.StatusCode)
	}
	return nil
}
