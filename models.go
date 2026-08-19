package gotopo

import (
	"encoding/json"
	"fmt"
)

// Properties contains CalTopo feature properties. The API is undocumented, so
// properties remain extensible while common accessors provide type safety.
type Properties map[string]any

func (p Properties) String(key string) string {
	v, _ := p[key].(string)
	return v
}

func (p Properties) Class() string { return p.String("class") }
func (p Properties) Title() string { return p.String("title") }

// Position is a GeoJSON coordinate in longitude, latitude order. Additional
// values such as elevation and timestamp are preserved.
type Position []float64

// Geometry is a GeoJSON geometry. Coordinates may be Position, []Position,
// [][]Position, or a value decoded from JSON.
type Geometry struct {
	Type        string `json:"type"`
	Coordinates any    `json:"coordinates,omitempty"`
	Size        int    `json:"size,omitempty"`
	Incremental bool   `json:"incremental,omitempty"`
}

// Feature is a CalTopo GeoJSON feature.
type Feature struct {
	Type       string     `json:"type,omitempty"`
	ID         string     `json:"id"`
	Properties Properties `json:"properties"`
	Geometry   *Geometry  `json:"geometry,omitempty"`
}

func (f Feature) Class() string { return f.Properties.Class() }
func (f Feature) Title() string { return f.Properties.Title() }

// Clone returns a deep copy suitable for mutation by a caller.
func (f Feature) Clone() (Feature, error) {
	b, err := json.Marshal(f)
	if err != nil {
		return Feature{}, fmt.Errorf("clone feature: %w", err)
	}
	var out Feature
	if err := json.Unmarshal(b, &out); err != nil {
		return Feature{}, fmt.Errorf("clone feature: %w", err)
	}
	return out, nil
}

type featureCollection struct {
	Type     string    `json:"type,omitempty"`
	Features []Feature `json:"features"`
}

type syncResult struct {
	IDs       map[string][]string `json:"ids,omitempty"`
	State     featureCollection   `json:"state"`
	Timestamp int64               `json:"timestamp"`
}

type responseEnvelope struct {
	Status    string          `json:"status"`
	Message   string          `json:"message,omitempty"`
	Result    json.RawMessage `json:"result"`
	Timestamp int64           `json:"timestamp,omitempty"`
}

// MapInfo describes a map or bookmark in an account.
type MapInfo struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Updated    int64  `json:"updated"`
	Type       string `json:"type"`
	Permission string `json:"permission,omitempty"`
	FolderID   string `json:"folderId,omitempty"`
	FolderName string `json:"folderName,omitempty"`
	Locked     bool   `json:"locked,omitempty"`
}

// AccountMapList groups a map list under an account.
type AccountMapList struct {
	AccountTitle string    `json:"accountTitle"`
	Personal     bool      `json:"personal"`
	Maps         []MapInfo `json:"maps"`
}

// AccountFolder is a node in an account folder hierarchy.
type AccountFolder struct {
	ID         string          `json:"id"`
	Title      string          `json:"title"`
	Path       string          `json:"path"`
	Subfolders []AccountFolder `json:"subFolders,omitempty"`
}

// AccountFolders contains an account and its hierarchical and flat folder
// representations.
type AccountFolders struct {
	AccountID    string            `json:"accountId"`
	AccountTitle string            `json:"accountTitle"`
	Folders      []AccountFolder   `json:"folders"`
	PathsAndIDs  map[string]string `json:"pathsAndIds"`
}

// AccountData preserves the undocumented account response while exposing the
// collections used by account helper methods.
type AccountData struct {
	Features []Feature      `json:"features"`
	Accounts []Feature      `json:"accounts"`
	Rels     []Feature      `json:"rels"`
	Groups   []Feature      `json:"groups"`
	IDs      map[string]any `json:"ids"`
}

// DeleteTarget identifies a feature for a batch deletion.
type DeleteTarget struct {
	ID    string
	Class string
}
