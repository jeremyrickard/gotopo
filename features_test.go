package gotopo

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestAddMarkerPayloadAndCache(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/Marker" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(body))
		var feature Feature
		if err := json.Unmarshal([]byte(form.Get("json")), &feature); err != nil {
			t.Fatal(err)
		}
		point, ok := pointPosition(feature.Geometry.Coordinates)
		if !ok || point[0] != -120 || point[1] != 39 {
			t.Errorf("unexpected coordinates %#v", feature.Geometry.Coordinates)
		}
		feature.ID = "m1"
		feature.Properties["class"] = "Marker"
		response, _ := json.Marshal(map[string]any{"status": "ok", "result": feature})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(response)
	}))
	defer server.Close()

	client, err := NewClient(WithEndpoint(server.URL), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	client.mapID = "ABC"
	created, err := client.AddMarker(context.Background(), MarkerOptions{Latitude: 39, Longitude: -120, Title: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != "m1" {
		t.Fatalf("unexpected ID %q", created.ID)
	}
	if _, ok := client.features[featureKey("Marker", "m1")]; !ok {
		t.Fatal("created marker was not cached")
	}
}

func TestValidateAndFixPoints(t *testing.T) {
	geometry, err := validateGeometry(Geometry{
		Type: "LineString", Coordinates: []Position{{39, -120}, {40, -121}},
	}, ValidateAndFixPoints)
	if err != nil {
		t.Fatal(err)
	}
	points, _ := positions(geometry.Coordinates)
	if points[0][0] != -120 || points[0][1] != 39 {
		t.Fatalf("coordinates were not swapped: %#v", points)
	}
}
