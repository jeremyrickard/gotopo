package gotopo

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFlushPreservesItemsQueuedDuringRequest(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v0/map/ABC/save":
			close(started)
			<-release
			_, _ = io.WriteString(w, `{"status":"ok","result":{}}`)
		case "/since/0":
			_, _ = io.WriteString(w, `{"status":"ok","result":{"ids":{},"state":{"features":[]},"timestamp":1}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := NewClient(WithEndpoint(server.URL), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	client.mapID = "ABC"
	client.queued["Marker"] = []Feature{{Properties: Properties{"title": "first"}}}

	done := make(chan error, 1)
	go func() { done <- client.Flush(context.Background()) }()
	<-started
	client.mu.Lock()
	client.queued["Marker"] = append(client.queued["Marker"], Feature{Properties: Properties{"title": "second"}})
	client.mu.Unlock()
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	client.mu.RLock()
	defer client.mu.RUnlock()
	if len(client.queued["Marker"]) != 1 || client.queued["Marker"][0].Title() != "second" {
		t.Fatalf("concurrently queued item was lost: %#v", client.queued)
	}
}
