package gotopo

import (
	"context"
	"math"
	"testing"
)

func TestGetBounds(t *testing.T) {
	client, _ := NewClient()
	client.mapID = "ABC"
	line := Feature{
		ID: "line", Properties: Properties{"class": "Shape", "title": "line"},
		Geometry: &Geometry{Type: "LineString", Coordinates: []Position{{-121, 39}, {-120, 40}}},
	}
	pad := 0.1
	got, err := client.GetBounds(context.Background(), []FeatureRef{{Feature: &line}}, BoundsOptions{PadPercent: &pad})
	if err != nil {
		t.Fatal(err)
	}
	want := [4]float64{-121.1, 38.9, -119.9, 40.1}
	for i := range got {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Fatalf("bounds[%d] got %f want %f", i, got[i], want[i])
		}
	}
}

func TestCropNoDraw(t *testing.T) {
	client, _ := NewClient()
	client.mapID = "ABC"
	target := Feature{
		ID: "line", Properties: Properties{"class": "Shape", "title": "line"},
		Geometry: &Geometry{Type: "LineString", Coordinates: []Position{{-2, 0}, {2, 0}}},
	}
	boundary := Feature{
		ID: "poly", Properties: Properties{"class": "Shape", "title": "poly"},
		Geometry: &Geometry{Type: "Polygon", Coordinates: [][]Position{{
			{-1, -1}, {1, -1}, {1, 1}, {-1, 1}, {-1, -1},
		}}},
	}
	result, err := client.Crop(context.Background(), FeatureRef{Feature: &target}, FeatureRef{Feature: &boundary}, CropOptions{Beyond: 1e-9, NoDraw: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Coordinates) != 1 || len(result.Coordinates[0][0]) != 2 {
		t.Fatalf("unexpected crop result %#v", result.Coordinates)
	}
}

func TestPreserveExtraCoordinates(t *testing.T) {
	original := []Position{{-2, 0, 10, 100}, {0, 0, 20, 200}, {2, 0, 30, 300}}
	got := preserveExtraCoordinates([]Position{{-1, 0}, {0, 0}, {1, 0}}, original)
	if len(got[0]) != 4 || got[0][3] != 100 {
		t.Fatalf("generated first point did not inherit timestamp: %#v", got[0])
	}
	if len(got[1]) != 4 || got[1][2] != 20 || got[1][3] != 200 {
		t.Fatalf("existing point did not preserve metadata: %#v", got[1])
	}
	if len(got[2]) != 4 || got[2][3] != 300 {
		t.Fatalf("generated final point did not inherit timestamp: %#v", got[2])
	}
}
