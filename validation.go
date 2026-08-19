package gotopo

import (
	"encoding/json"
	"fmt"
	"math"
)

func validateGeometry(geometry Geometry, mode PointValidation) (Geometry, error) {
	if mode == DisablePointValidation || geometry.Coordinates == nil {
		return geometry, nil
	}
	switch geometry.Type {
	case "Point":
		point, ok := pointPosition(geometry.Coordinates)
		if !ok {
			return Geometry{}, fmt.Errorf("gotopo: invalid point coordinates")
		}
		points, err := validatePointList([]Position{point}, mode)
		if err != nil {
			return Geometry{}, err
		}
		geometry.Coordinates = points[0]
	case "LineString":
		points, ok := positions(geometry.Coordinates)
		if !ok {
			return Geometry{}, fmt.Errorf("gotopo: invalid line coordinates")
		}
		points, err := validatePointList(points, mode)
		if err != nil {
			return Geometry{}, err
		}
		geometry.Coordinates = points
	case "Polygon":
		rings, ok := polygonPositions(geometry.Coordinates)
		if !ok {
			return Geometry{}, fmt.Errorf("gotopo: invalid polygon coordinates")
		}
		for i, ring := range rings {
			validated, err := validatePointList(ring, mode)
			if err != nil {
				return Geometry{}, err
			}
			rings[i] = validated
		}
		geometry.Coordinates = rings
	default:
		if _, err := json.Marshal(geometry.Coordinates); err != nil {
			return Geometry{}, fmt.Errorf("gotopo: invalid geometry coordinates: %w", err)
		}
	}
	return geometry, nil
}

func validatePointList(points []Position, mode PointValidation) ([]Position, error) {
	var swapped, valid bool
	for _, point := range points {
		if len(point) < 2 {
			return nil, fmt.Errorf("gotopo: coordinate has fewer than two values")
		}
		if math.Abs(point[1]) > 90 {
			swapped = true
		} else if math.Abs(point[0]) >= 90 && math.Abs(point[0]) <= 180 {
			valid = true
		}
	}
	if swapped && valid {
		return nil, fmt.Errorf("gotopo: coordinate list mixes obviously valid and longitude/latitude-swapped points")
	}
	out := make([]Position, len(points))
	for i, point := range points {
		out[i] = append(Position(nil), point...)
		if swapped && mode == ValidateAndFixPoints {
			out[i][0], out[i][1] = out[i][1], out[i][0]
		}
	}
	return out, nil
}
