package model

// GraphEdge represents a dependency relationship as a graph edge.
type GraphEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
}

// GraphStats holds aggregate bead counts by status.
type GraphStats struct {
	TotalOpen       int `json:"total_open"`
	TotalInProgress int `json:"total_in_progress"`
	TotalBlocked    int `json:"total_blocked"`
	TotalClosed     int `json:"total_closed"`
	TotalDeferred   int `json:"total_deferred"`
}

// GraphResponse is the response for the graph visualization endpoint.
type GraphResponse struct {
	Nodes []*Bead      `json:"nodes"`
	Edges []*GraphEdge `json:"edges"`
	Stats *GraphStats  `json:"stats"`
}
