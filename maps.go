package gotopo

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// OpenMapOptions controls map opening and optional background synchronization.
type OpenMapOptions struct {
	// DisableBackgroundSync opts out of the upstream-compatible default.
	DisableBackgroundSync bool
}

// OpenMap associates the client with an existing map and performs an initial,
// blocking cache refresh.
func (c *Client) OpenMap(ctx context.Context, mapID string, opts OpenMapOptions) error {
	if len(mapID) < 3 || len(mapID) > 7 {
		return fmt.Errorf("gotopo: map ID must contain 3 to 7 characters")
	}
	c.StopSync()
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return ErrClosed
	}
	if c.mapID != "" {
		c.mu.Unlock()
		return fmt.Errorf("gotopo: map %s is already open", c.mapID)
	}
	c.mapID = mapID
	c.features = make(map[string]Feature)
	c.ids = make(map[string][]string)
	c.lastServerSync = 0
	c.lastLocalSync = time.Time{}
	c.mu.Unlock()

	if err := c.Refresh(ctx, true); err != nil {
		c.mu.Lock()
		c.mapID = ""
		c.mu.Unlock()
		return fmt.Errorf("gotopo: open map %s: %w", mapID, err)
	}
	if !opts.DisableBackgroundSync {
		return c.StartSync(context.Background())
	}
	return nil
}

// NewMapOptions describes a newly created map.
type NewMapOptions struct {
	Title     string
	AccountID string
	FolderID  string
	Mode      string
	Sharing   string
}

// CreateMap creates a map, opens it, and removes the server-required dummy
// marker. AccountID defaults to the client's configured account.
func (c *Client) CreateMap(ctx context.Context, opts NewMapOptions) (string, error) {
	if opts.Title == "" {
		opts.Title = "newMap"
	}
	if opts.Mode == "" {
		opts.Mode = "cal"
	}
	if opts.Mode != "cal" && opts.Mode != "sar" {
		return "", fmt.Errorf("gotopo: map mode must be cal or sar")
	}
	if opts.Sharing == "" {
		opts.Sharing = "SECRET"
	}
	switch opts.Sharing {
	case "PRIVATE", "SECRET", "URL", "PUBLIC":
	default:
		return "", fmt.Errorf("gotopo: invalid map sharing mode %q", opts.Sharing)
	}
	accountID := opts.AccountID
	if accountID == "" {
		accountID = c.credentials.AccountID
	}
	if accountID == "" {
		return "", fmt.Errorf("gotopo: account ID is required to create a map")
	}
	const dummyID = "11111111-1111-1111-1111-111111111111"
	properties := Properties{
		"title": opts.Title, "mode": opts.Mode,
		"mapConfig": map[string]any{"activeLayers": [][]any{{"mbt", 1}}},
		"sharing":   opts.Sharing,
	}
	if opts.FolderID != "" {
		properties["folderId"] = opts.FolderID
	}
	payload := map[string]any{
		"properties": properties,
		"state": featureCollection{Type: "FeatureCollection", Features: []Feature{{
			Type: "Feature", ID: dummyID, Properties: Properties{"title": "NewMapDummyMarker"},
			Geometry: &Geometry{Type: "Point", Coordinates: Position{-120, 39}},
		}}},
	}
	var created struct {
		ID string `json:"id"`
	}
	path := "/api/v1/acct/" + accountID + "/CollaborativeMap"
	if err := c.do(ctx, requestSpec{method: http.MethodPost, path: path, payload: payload}, &created); err != nil {
		return "", err
	}
	mapID := lastPathSegment(created.ID)
	if mapID == "" {
		return "", fmt.Errorf("gotopo: create map response contained no map ID")
	}
	if err := c.OpenMap(ctx, mapID, OpenMapOptions{}); err != nil {
		return "", err
	}
	if _, err := c.DeleteFeature(ctx, DeleteTarget{ID: dummyID, Class: "Marker"}); err != nil {
		return "", fmt.Errorf("gotopo: delete new-map dummy marker: %w", err)
	}
	return mapID, nil
}

func lastPathSegment(value string) string {
	for len(value) > 0 && value[len(value)-1] == '/' {
		value = value[:len(value)-1]
	}
	for i := len(value) - 1; i >= 0; i-- {
		if value[i] == '/' {
			return value[i+1:]
		}
	}
	return value
}

// CloseMap stops synchronization and returns the client to a mapless state.
func (c *Client) CloseMap() {
	c.StopSync()
	c.mu.Lock()
	c.mapID = ""
	c.features = make(map[string]Feature)
	c.ids = make(map[string][]string)
	c.queued = make(map[string][]Feature)
	c.lastServerSync = 0
	c.lastLocalSync = time.Time{}
	handler := c.events.MapClosed
	c.mu.Unlock()
	if handler != nil {
		handler()
	}
}
