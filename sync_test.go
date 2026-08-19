package gotopo

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestApplySyncAndQuery(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	client.mapID = "ABC"
	client.applySync(syncResult{
		IDs: map[string][]string{"Assignment": {"a1"}},
		State: featureCollection{Features: []Feature{{
			ID: "a1", Properties: Properties{
				"class": "Assignment", "letter": "A", "number": "12", "title": "",
			},
			Geometry: &Geometry{Type: "LineString", Coordinates: []Position{
				{-120, 39, 0, 1}, {-120.5, 39.5, 0, 2}, {-121, 40, 0, 3},
			}},
		}}},
		Timestamp: 1000,
	})
	client.lastLocalSync = time.Now()

	got, err := client.GetFeature(context.Background(), FeatureFilter{Title: "A", LetterOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if got.Title() != "A 12" {
		t.Fatalf("assignment title not normalized: %q", got.Title())
	}

	client.applySync(syncResult{
		IDs: map[string][]string{"Assignment": {"a1"}},
		State: featureCollection{Features: []Feature{{
			ID: "a1", Properties: Properties{"class": "Assignment", "nop": true},
			Geometry: &Geometry{Type: "LineString", Incremental: true, Coordinates: []Position{
				{-121, 40, 0, 3}, {-121.5, 40.5, 0, 4}, {-122, 41, 0, 5},
			}},
		}}},
		Timestamp: 2000,
	})
	got, err = client.GetFeature(context.Background(), FeatureFilter{ID: "a1"})
	if err != nil {
		t.Fatal(err)
	}
	points, ok := positions(got.Geometry.Coordinates)
	if !ok || len(points) != 5 {
		t.Fatalf("incremental geometry not merged: %#v", got.Geometry.Coordinates)
	}

	client.applySync(syncResult{IDs: map[string][]string{"Assignment": {}}, Timestamp: 3000})
	_, err = client.GetFeature(context.Background(), FeatureFilter{ID: "a1"})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected deleted feature to be absent, got %v", err)
	}
}

func TestAmbiguousTitle(t *testing.T) {
	client, _ := NewClient()
	client.mapID = "ABC"
	client.lastLocalSync = time.Now()
	client.features[featureKey("Marker", "1")] = Feature{ID: "1", Properties: Properties{"class": "Marker", "title": "same"}}
	client.features[featureKey("Shape", "2")] = Feature{ID: "2", Properties: Properties{"class": "Shape", "title": "SAME"}}
	_, err := client.GetFeatures(context.Background(), FeatureFilter{Title: "same"})
	if !errors.Is(err, ErrAmbiguousMatch) {
		t.Fatalf("expected ambiguous match, got %v", err)
	}
}

func TestEventFeatureDoesNotAliasCache(t *testing.T) {
	var event Feature
	client, err := NewClient(WithEventHandlers(EventHandlers{
		FeatureAdded: func(feature Feature) { event = feature },
	}))
	if err != nil {
		t.Fatal(err)
	}
	client.mapID = "ABC"
	client.applySync(syncResult{
		IDs: map[string][]string{"Marker": {"m1"}},
		State: featureCollection{Features: []Feature{{
			ID: "m1", Properties: Properties{"class": "Marker", "title": "original"},
		}}},
		Timestamp: 1,
	})
	event.Properties["title"] = "mutated by handler"
	client.mu.RLock()
	cached := client.features[featureKey("Marker", "m1")].Title()
	client.mu.RUnlock()
	if cached != "original" {
		t.Fatalf("handler feature mutated cache: %q", cached)
	}
}
