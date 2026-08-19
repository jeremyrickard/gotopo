package gotopo

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	sf "github.com/peterstace/simplefeatures/geom"
)

// FeatureRef identifies a feature by ID, title, or a supplied value.
type FeatureRef struct {
	ID      string
	Title   string
	Feature *Feature
}

type CutOptions struct {
	// KeepCutter opts out of deleting the cutter after a successful cut.
	KeepCutter bool
	// DisableResultSuffix opts out of suffixing additional result features.
	DisableResultSuffix bool
}

// Cut subtracts a line or polygon cutter from a line or polygon target and
// persists all resulting parts.
func (c *Client) Cut(ctx context.Context, targetRef, cutterRef FeatureRef, opts CutOptions) ([]string, error) {
	target, err := c.resolveFeature(ctx, targetRef)
	if err != nil {
		return nil, fmt.Errorf("gotopo: resolve cut target: %w", err)
	}
	cutter, err := c.resolveFeature(ctx, cutterRef)
	if err != nil {
		return nil, fmt.Errorf("gotopo: resolve cutter: %w", err)
	}
	targetGeom, err := simpleGeometry(target)
	if err != nil {
		return nil, err
	}
	cutterGeom, err := simpleGeometry(cutter)
	if err != nil {
		return nil, err
	}
	if !sf.Intersects(targetGeom, cutterGeom) {
		return nil, fmt.Errorf("gotopo: target and cutter do not intersect")
	}
	// A zero-width line does not alter polygon area in an OGC difference.
	// A sub-nanodegree buffer creates the same split topology while keeping
	// coordinate movement far below CalTopo's precision.
	if targetGeom.IsPolygon() && cutterGeom.IsLineString() {
		cutterGeom, err = sf.Buffer(cutterGeom, 1e-12)
		if err != nil {
			return nil, fmt.Errorf("gotopo: buffer line cutter: %w", err)
		}
	}
	result, err := sf.Difference(targetGeom, cutterGeom)
	if err != nil {
		return nil, fmt.Errorf("gotopo: cut geometry: %w", err)
	}
	ids, err := c.persistGeometryParts(ctx, target, result, !opts.DisableResultSuffix)
	if err != nil {
		return nil, err
	}
	if !opts.KeepCutter {
		if _, err := c.DeleteFeature(ctx, DeleteTarget{ID: cutter.ID, Class: cutter.Class()}); err != nil {
			return nil, fmt.Errorf("gotopo: delete cutter: %w", err)
		}
	}
	return ids, nil
}

type ExpandOptions struct {
	// KeepSecond opts out of deleting the second polygon after expansion.
	KeepSecond bool
}

// Expand unions two intersecting polygons and writes the result to target.
func (c *Client) Expand(ctx context.Context, targetRef, secondRef FeatureRef, opts ExpandOptions) (Feature, error) {
	target, err := c.resolveFeature(ctx, targetRef)
	if err != nil {
		return Feature{}, err
	}
	second, err := c.resolveFeature(ctx, secondRef)
	if err != nil {
		return Feature{}, err
	}
	a, err := simpleGeometry(target)
	if err != nil {
		return Feature{}, err
	}
	b, err := simpleGeometry(second)
	if err != nil {
		return Feature{}, err
	}
	if !a.IsPolygon() || !b.IsPolygon() {
		return Feature{}, fmt.Errorf("gotopo: expand requires two polygons")
	}
	if !sf.Intersects(a, b) {
		return Feature{}, fmt.Errorf("gotopo: polygons do not intersect")
	}
	union, err := sf.Union(a, b)
	if err != nil {
		return Feature{}, fmt.Errorf("gotopo: union polygons: %w", err)
	}
	parts := desiredParts(union, true)
	if len(parts) != 1 {
		return Feature{}, fmt.Errorf("gotopo: polygon union produced %d parts", len(parts))
	}
	geometry, err := geometryFromSimple(parts[0])
	if err != nil {
		return Feature{}, err
	}
	edited, err := c.EditFeature(ctx, EditFeatureOptions{ID: target.ID, Class: target.Class(), Geometry: &geometry})
	if err != nil {
		return Feature{}, err
	}
	if !opts.KeepSecond {
		if _, err := c.DeleteFeature(ctx, DeleteTarget{ID: second.ID, Class: second.Class()}); err != nil {
			return Feature{}, err
		}
	}
	return edited, nil
}

type CropOptions struct {
	Beyond            float64
	DeleteBoundary    bool
	SuffixResults     bool
	DrawSizedBoundary bool
	NoDraw            bool
}

// CropResult contains either persisted feature IDs or, with NoDraw, result
// line/polygon coordinate sets.
type CropResult struct {
	FeatureIDs  []string
	Coordinates [][][]Position
}

// Crop intersects a target with a buffered line or polygon boundary.
func (c *Client) Crop(ctx context.Context, targetRef, boundaryRef FeatureRef, opts CropOptions) (CropResult, error) {
	target, err := c.resolveFeature(ctx, targetRef)
	if err != nil {
		return CropResult{}, err
	}
	boundary, err := c.resolveFeature(ctx, boundaryRef)
	if err != nil {
		return CropResult{}, err
	}
	targetGeom, err := simpleGeometry(target)
	if err != nil {
		return CropResult{}, err
	}
	boundaryGeom, err := simpleGeometry(boundary)
	if err != nil {
		return CropResult{}, err
	}
	if !boundaryGeom.IsPolygon() && !boundaryGeom.IsLineString() {
		return CropResult{}, fmt.Errorf("gotopo: crop boundary must be a polygon or line")
	}
	beyond := opts.Beyond
	if beyond == 0 {
		beyond = 0.0001
	}
	boundaryGeom, err = sf.Buffer(boundaryGeom, beyond)
	if err != nil {
		return CropResult{}, fmt.Errorf("gotopo: buffer crop boundary: %w", err)
	}
	if opts.DrawSizedBoundary {
		geometry, err := geometryFromSimple(boundaryGeom)
		if err != nil {
			return CropResult{}, err
		}
		if _, err := c.addFeature(ctx, "Shape", Feature{
			Properties: cloneProperties(target.Properties), Geometry: &geometry,
		}, false); err != nil {
			return CropResult{}, fmt.Errorf("gotopo: draw sized boundary: %w", err)
		}
	}
	if !sf.Intersects(targetGeom, boundaryGeom) {
		return CropResult{}, fmt.Errorf("gotopo: target and crop boundary do not intersect")
	}
	result, err := sf.Intersection(targetGeom, boundaryGeom)
	if err != nil {
		return CropResult{}, fmt.Errorf("gotopo: crop geometry: %w", err)
	}
	if opts.NoDraw {
		coordinates, err := coordinateParts(result)
		return CropResult{Coordinates: coordinates}, err
	}
	ids, err := c.persistGeometryParts(ctx, target, result, opts.SuffixResults)
	if err != nil {
		return CropResult{}, err
	}
	if opts.DeleteBoundary {
		if _, err := c.DeleteFeature(ctx, DeleteTarget{ID: boundary.ID, Class: boundary.Class()}); err != nil {
			return CropResult{}, err
		}
	}
	return CropResult{FeatureIDs: ids}, nil
}

type BoundsOptions struct {
	PadDegrees float64
	PadPercent *float64
}

// GetBounds returns min longitude, min latitude, max longitude, max latitude.
func (c *Client) GetBounds(ctx context.Context, refs []FeatureRef, opts BoundsOptions) ([4]float64, error) {
	if len(refs) == 0 {
		return [4]float64{}, fmt.Errorf("gotopo: at least one feature is required")
	}
	bounds := [4]float64{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
	for _, ref := range refs {
		feature, err := c.resolveFeature(ctx, ref)
		if err != nil {
			return [4]float64{}, err
		}
		geometry, err := simpleGeometry(feature)
		if err != nil {
			return [4]float64{}, err
		}
		envelope := geometry.Envelope()
		min, max, ok := envelope.MinMaxXYs()
		if !ok {
			return [4]float64{}, fmt.Errorf("gotopo: feature %s has empty geometry", feature.ID)
		}
		bounds[0] = math.Min(bounds[0], min.X)
		bounds[1] = math.Min(bounds[1], min.Y)
		bounds[2] = math.Max(bounds[2], max.X)
		bounds[3] = math.Max(bounds[3], max.Y)
	}
	pad := opts.PadDegrees
	if pad == 0 && opts.PadPercent == nil {
		pad = 0.0001
	}
	if opts.PadPercent != nil {
		ratio := *opts.PadPercent
		if ratio >= 1 {
			ratio /= 100
		}
		pad = math.Max(bounds[2]-bounds[0], bounds[3]-bounds[1]) * ratio
	}
	return [4]float64{bounds[0] - pad, bounds[1] - pad, bounds[2] + pad, bounds[3] + pad}, nil
}

func (c *Client) resolveFeature(ctx context.Context, ref FeatureRef) (Feature, error) {
	if ref.Feature != nil {
		return ref.Feature.Clone()
	}
	if ref.ID != "" {
		return c.GetFeature(ctx, FeatureFilter{ID: ref.ID})
	}
	if ref.Title != "" {
		return c.GetFeature(ctx, FeatureFilter{
			Title: ref.Title, ExcludeClasses: []string{"Folder", "OperationalPeriod"},
		})
	}
	return Feature{}, fmt.Errorf("gotopo: empty feature reference")
}

func simpleGeometry(feature Feature) (sf.Geometry, error) {
	if feature.Geometry == nil {
		return sf.Geometry{}, fmt.Errorf("gotopo: feature %s has no geometry", feature.ID)
	}
	b, err := json.Marshal(feature.Geometry)
	if err != nil {
		return sf.Geometry{}, fmt.Errorf("gotopo: encode geometry: %w", err)
	}
	geometry, err := sf.UnmarshalGeoJSON(b)
	if err != nil {
		return sf.Geometry{}, fmt.Errorf("gotopo: decode feature %s geometry: %w", feature.ID, err)
	}
	return geometry.Force2D(), nil
}

func geometryFromSimple(geometry sf.Geometry) (Geometry, error) {
	b, err := geometry.MarshalJSON()
	if err != nil {
		return Geometry{}, fmt.Errorf("gotopo: encode geometry result: %w", err)
	}
	var result Geometry
	if err := json.Unmarshal(b, &result); err != nil {
		return Geometry{}, fmt.Errorf("gotopo: decode geometry result: %w", err)
	}
	return result, nil
}

func desiredParts(result sf.Geometry, polygon bool) []sf.Geometry {
	parts := make([]sf.Geometry, 0)
	for _, part := range result.Dump() {
		if polygon && part.IsPolygon() {
			parts = append(parts, part)
		}
		if !polygon && part.IsLineString() {
			parts = append(parts, part)
		}
	}
	return parts
}

func (c *Client) persistGeometryParts(ctx context.Context, target Feature, result sf.Geometry, suffix bool) ([]string, error) {
	polygon := target.Geometry != nil && target.Geometry.Type == "Polygon"
	parts := desiredParts(result, polygon)
	if len(parts) == 0 {
		return nil, fmt.Errorf("gotopo: geometry operation produced no compatible parts")
	}
	ids := make([]string, 0, len(parts))
	for i, part := range parts {
		geometry, err := geometryFromSimple(part)
		if err != nil {
			return nil, err
		}
		if !polygon && target.Geometry != nil {
			if resultPoints, ok := positions(geometry.Coordinates); ok {
				if originalPoints, ok := positions(target.Geometry.Coordinates); ok {
					geometry.Coordinates = preserveExtraCoordinates(resultPoints, originalPoints)
				}
			}
		}
		if i == 0 {
			edited, err := c.EditFeature(ctx, EditFeatureOptions{
				ID: target.ID, Class: target.Class(), Geometry: &geometry,
			})
			if err != nil {
				return nil, err
			}
			ids = append(ids, edited.ID)
			continue
		}
		properties := cloneProperties(target.Properties)
		if suffix {
			baseKey := "title"
			if target.Class() == "Assignment" {
				baseKey = "letter"
			}
			base := unsuffixed(properties.String(baseKey))
			properties[baseKey] = c.nextSuffix(base)
			if target.Class() == "Assignment" {
				properties["title"] = strings.TrimSpace(properties.String("letter") + " " + properties.String("number"))
			}
		}
		created, err := c.addFeature(ctx, target.Class(), Feature{
			Type: target.Type, Properties: properties, Geometry: &geometry,
		}, false)
		if err != nil {
			return nil, err
		}
		ids = append(ids, created.ID)
	}
	return ids, nil
}

func preserveExtraCoordinates(points, original []Position) []Position {
	if len(original) == 0 || len(original[0]) <= 2 || len(original[len(original)-1]) <= 2 {
		return points
	}
	out := make([]Position, len(points))
	for i, point := range points {
		out[i] = append(Position(nil), point...)
		for _, candidate := range original {
			if len(point) >= 2 && len(candidate) >= 2 && point[0] == candidate[0] && point[1] == candidate[1] {
				out[i] = append(Position(nil), candidate...)
				break
			}
		}
		if len(out[i]) == 2 && len(original[0]) >= 4 {
			timestamp := original[0][3]
			if i == len(points)-1 {
				timestamp = original[len(original)-1][3]
			}
			out[i] = append(out[i], 0, timestamp)
		}
	}
	return out
}

func (c *Client) nextSuffix(base string) string {
	used := make(map[int]bool)
	c.mu.RLock()
	for _, feature := range c.features {
		value := feature.Title()
		if feature.Class() == "Assignment" && feature.Properties.String("letter") != "" {
			value = feature.Properties.String("letter")
		}
		if !strings.HasPrefix(value, base+":") {
			continue
		}
		var suffix int
		if _, err := fmt.Sscanf(strings.TrimPrefix(value, base+":"), "%d", &suffix); err == nil {
			used[suffix] = true
		}
	}
	c.mu.RUnlock()
	for suffix := 1; suffix < 100; suffix++ {
		if !used[suffix] {
			return fmt.Sprintf("%s:%d", base, suffix)
		}
	}
	return base + ":100"
}

func unsuffixed(value string) string {
	index := strings.LastIndex(value, ":")
	if index < 0 {
		return value
	}
	var suffix int
	if _, err := fmt.Sscanf(value[index+1:], "%d", &suffix); err == nil {
		return value[:index]
	}
	return value
}

func coordinateParts(result sf.Geometry) ([][][]Position, error) {
	polygon := result.IsPolygon() || result.IsMultiPolygon()
	parts := desiredParts(result, polygon)
	out := make([][][]Position, 0, len(parts))
	for _, part := range parts {
		geometry, err := geometryFromSimple(part)
		if err != nil {
			return nil, err
		}
		if part.IsPolygon() {
			rings, ok := polygonPositions(geometry.Coordinates)
			if !ok {
				return nil, fmt.Errorf("gotopo: invalid polygon result")
			}
			out = append(out, rings)
		} else {
			line, ok := positions(geometry.Coordinates)
			if !ok {
				return nil, fmt.Errorf("gotopo: invalid line result")
			}
			out = append(out, [][]Position{line})
		}
	}
	return out, nil
}
