package gotopo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

type CreateOptions struct {
	Queue bool
}

type FolderOptions struct {
	Title        string
	Visible      *bool
	LabelVisible *bool
	CreateOptions
}

func (c *Client) AddFolder(ctx context.Context, opts FolderOptions) (Feature, error) {
	if opts.Title == "" {
		opts.Title = "New Folder"
	}
	visible, labelVisible := true, true
	if opts.Visible != nil {
		visible = *opts.Visible
	}
	if opts.LabelVisible != nil {
		labelVisible = *opts.LabelVisible
	}
	return c.addFeature(ctx, "Folder", Feature{Properties: Properties{
		"title": opts.Title, "visible": visible, "labelVisible": labelVisible,
	}}, opts.Queue)
}

type MarkerOptions struct {
	Latitude    float64
	Longitude   float64
	Title       string
	Description string
	Color       string
	Symbol      string
	Rotation    *float64
	FolderID    string
	Size        float64
	CreateOptions
}

func (c *Client) AddMarker(ctx context.Context, opts MarkerOptions) (Feature, error) {
	if opts.Title == "" {
		opts.Title = "New Marker"
	}
	if opts.Color == "" {
		opts.Color = "#FF0000"
	}
	if opts.Symbol == "" {
		opts.Symbol = "point"
	}
	if opts.Size == 0 {
		opts.Size = 1
	}
	properties := Properties{
		"marker-color": opts.Color, "marker-symbol": opts.Symbol,
		"marker-size": opts.Size, "marker-rotation": opts.Rotation,
		"marker-visibility": "visible", "title": opts.Title,
		"description": opts.Description,
	}
	setOptional(properties, "folderId", opts.FolderID)
	return c.addFeature(ctx, "Marker", Feature{
		Type: "Feature", Properties: properties,
		Geometry: &Geometry{Type: "Point", Coordinates: Position{opts.Longitude, opts.Latitude}},
	}, opts.Queue)
}

type LineOptions struct {
	Points      []Position
	Title       string
	Description string
	Width       float64
	Opacity     *float64
	Color       string
	Pattern     string
	FolderID    string
	CreateOptions
}

func (c *Client) AddLine(ctx context.Context, opts LineOptions) (Feature, error) {
	if opts.Title == "" {
		opts.Title = "New Line"
	}
	if opts.Width == 0 {
		opts.Width = 2
	}
	opacity := 1.0
	if opts.Opacity != nil {
		opacity = *opts.Opacity
	}
	if opts.Color == "" {
		opts.Color = "#FF0000"
	}
	if opts.Pattern == "" {
		opts.Pattern = "solid"
	}
	if len(opts.Points) < 2 {
		return Feature{}, fmt.Errorf("gotopo: a line requires at least two points")
	}
	properties := Properties{
		"title": opts.Title, "description": opts.Description,
		"stroke-width": opts.Width, "stroke-opacity": opacity,
		"stroke": opts.Color, "pattern": opts.Pattern,
	}
	setOptional(properties, "folderId", opts.FolderID)
	return c.addFeature(ctx, "Shape", Feature{
		Properties: properties,
		Geometry:   &Geometry{Type: "LineString", Coordinates: opts.Points},
	}, opts.Queue)
}

type PolygonOptions struct {
	Points        []Position
	Title         string
	Description   string
	StrokeOpacity *float64
	StrokeWidth   float64
	FillOpacity   *float64
	Stroke        string
	Fill          string
	FolderID      string
	CreateOptions
}

func (c *Client) AddPolygon(ctx context.Context, opts PolygonOptions) (Feature, error) {
	if opts.Title == "" {
		opts.Title = "New Shape"
	}
	if opts.StrokeWidth == 0 {
		opts.StrokeWidth = 2
	}
	strokeOpacity, fillOpacity := 1.0, 0.1
	if opts.StrokeOpacity != nil {
		strokeOpacity = *opts.StrokeOpacity
	}
	if opts.FillOpacity != nil {
		fillOpacity = *opts.FillOpacity
	}
	if opts.Stroke == "" {
		opts.Stroke = "#FF0000"
	}
	if opts.Fill == "" {
		opts.Fill = "#FF0000"
	}
	if len(opts.Points) < 3 {
		return Feature{}, fmt.Errorf("gotopo: a polygon requires at least three points")
	}
	properties := Properties{
		"title": opts.Title, "description": opts.Description,
		"stroke-width": opts.StrokeWidth, "stroke-opacity": strokeOpacity,
		"stroke": opts.Stroke, "fill": opts.Fill, "fill-opacity": fillOpacity,
	}
	setOptional(properties, "folderId", opts.FolderID)
	return c.addFeature(ctx, "Shape", Feature{
		Properties: properties,
		Geometry:   &Geometry{Type: "Polygon", Coordinates: [][]Position{opts.Points}},
	}, opts.Queue)
}

func (c *Client) addFeature(ctx context.Context, class string, feature Feature, queued bool) (Feature, error) {
	if _, err := c.requireMap(); err != nil {
		return Feature{}, err
	}
	if feature.Geometry != nil {
		geometry, err := validateGeometry(*feature.Geometry, c.pointValidation)
		if err != nil {
			return Feature{}, err
		}
		feature.Geometry = &geometry
	}
	if queued {
		c.mu.Lock()
		c.queued[class] = append(c.queued[class], feature)
		c.mu.Unlock()
		return feature, nil
	}
	path, err := c.mapPath(class, "")
	if err != nil {
		return Feature{}, err
	}
	var created Feature
	if err := c.do(ctx, requestSpec{method: http.MethodPost, path: path, payload: feature}, &created); err != nil {
		return Feature{}, err
	}
	if created.ID == "" {
		return Feature{}, fmt.Errorf("gotopo: add %s response contained no feature ID", class)
	}
	if created.Properties == nil {
		created.Properties = feature.Properties
	}
	if created.Properties.Class() == "" {
		created.Properties["class"] = class
	}
	if created.Geometry == nil {
		created.Geometry = feature.Geometry
	}
	c.upsertFeature(created)
	return created.Clone()
}

// Flush saves all deferred feature creations. The queue is cleared only after
// a successful response.
func (c *Client) Flush(ctx context.Context) error {
	mapID, err := c.requireMap()
	if err != nil {
		return err
	}
	c.mu.Lock()
	if len(c.queued) == 0 {
		c.mu.Unlock()
		return nil
	}
	captured := c.queued
	c.queued = make(map[string][]Feature)
	c.mu.Unlock()

	payload := make(map[string][]Feature, len(captured))
	for class, features := range captured {
		payload[class] = append([]Feature(nil), features...)
	}
	path := "/api/v0/map/" + mapID + "/save"
	if err := c.do(ctx, requestSpec{method: http.MethodPost, path: path, payload: payload}, nil); err != nil {
		c.mu.Lock()
		for class, features := range captured {
			c.queued[class] = append(append([]Feature(nil), features...), c.queued[class]...)
		}
		c.mu.Unlock()
		return err
	}
	return c.Refresh(ctx, true)
}

type EditFeatureOptions struct {
	ID         string
	Class      string
	Title      string
	Letter     string
	FolderID   *string
	Properties Properties
	Geometry   *Geometry
}

func (c *Client) EditFeature(ctx context.Context, opts EditFeatureOptions) (Feature, error) {
	filter := FeatureFilter{ID: opts.ID, Class: opts.Class}
	if opts.ID == "" {
		if opts.Class == "" {
			return Feature{}, fmt.Errorf("gotopo: class is required when editing without an ID")
		}
		if opts.Letter != "" {
			filter.Title = opts.Letter
			filter.LetterOnly = true
		} else if opts.Title != "" {
			filter.Title = opts.Title
		} else {
			return Feature{}, fmt.Errorf("gotopo: title or letter is required when editing without an ID")
		}
	}
	feature, err := c.GetFeature(ctx, filter)
	if err != nil {
		return Feature{}, err
	}
	if opts.Class == "" {
		opts.Class = feature.Class()
	}
	properties := cloneProperties(feature.Properties)
	for key, value := range opts.Properties {
		properties[key] = value
	}
	if opts.ID != "" && opts.Title != "" {
		properties["title"] = opts.Title
	}
	if opts.FolderID != nil {
		properties["folderId"] = *opts.FolderID
	}
	if opts.Class == "Assignment" {
		properties["title"] = strings.TrimSpace(properties.String("letter") + " " + properties.String("number"))
	}
	edited := Feature{Type: "Feature", ID: feature.ID, Properties: properties}
	if opts.Geometry != nil {
		geometry, err := validateGeometry(*opts.Geometry, c.pointValidation)
		if err != nil {
			return Feature{}, err
		}
		if line, ok := positions(geometry.Coordinates); ok {
			geometry.Size = len(line)
		}
		edited.Geometry = &geometry
	} else {
		edited.Geometry = feature.Geometry
	}
	path, err := c.mapPath(opts.Class, feature.ID)
	if err != nil {
		return Feature{}, err
	}
	var result Feature
	if err := c.do(ctx, requestSpec{method: http.MethodPost, path: path, payload: edited}, &result); err != nil {
		return Feature{}, err
	}
	if result.ID == "" {
		result = edited
	}
	if result.Properties == nil {
		result.Properties = edited.Properties
	}
	if result.Geometry == nil {
		result.Geometry = edited.Geometry
	}
	c.upsertFeature(result)
	return result.Clone()
}

func (c *Client) MoveMarker(ctx context.Context, id string, point Position) (Feature, error) {
	if len(point) < 2 {
		return Feature{}, fmt.Errorf("gotopo: marker position requires longitude and latitude")
	}
	return c.EditFeature(ctx, EditFeatureOptions{
		ID: id, Class: "Marker",
		Geometry: &Geometry{Type: "Point", Coordinates: Position{point[0], point[1], 0, 0}},
	})
}

func (c *Client) EditMarkerDescription(ctx context.Context, id, description string) (Feature, error) {
	return c.EditFeature(ctx, EditFeatureOptions{
		ID: id, Class: "Marker", Properties: Properties{"description": description},
	})
}

func (c *Client) DeleteFeature(ctx context.Context, target DeleteTarget) (string, error) {
	if target.ID == "" {
		return "", fmt.Errorf("gotopo: feature ID is required")
	}
	if target.Class == "" {
		feature, err := c.GetFeature(ctx, FeatureFilter{ID: target.ID})
		if err != nil {
			return "", fmt.Errorf("gotopo: determine feature class: %w", err)
		}
		target.Class = feature.Class()
	}
	path, err := c.mapPath(target.Class, target.ID)
	if err != nil {
		return "", err
	}
	var raw json.RawMessage
	if err := c.do(ctx, requestSpec{method: http.MethodDelete, path: path}, &raw); err != nil {
		return "", err
	}
	c.removeFeature(target.ID, target.Class)
	return target.ID, nil
}

func (c *Client) DeleteMarker(ctx context.Context, id string) (string, error) {
	return c.DeleteFeature(ctx, DeleteTarget{ID: id, Class: "Marker"})
}

// DeleteFeatures deletes targets with at most ten concurrent requests.
func (c *Client) DeleteFeatures(ctx context.Context, targets []DeleteTarget) error {
	if len(targets) == 0 {
		return fmt.Errorf("gotopo: no features to delete")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan DeleteTarget)
	errs := make(chan error, 1)
	var wg sync.WaitGroup
	worker := func() {
		defer wg.Done()
		for target := range jobs {
			if _, err := c.DeleteFeature(ctx, target); err != nil {
				select {
				case errs <- err:
					cancel()
				default:
				}
				return
			}
		}
	}
	workers := min(10, len(targets))
	wg.Add(workers)
	for range workers {
		go worker()
	}
sendLoop:
	for _, target := range targets {
		select {
		case jobs <- target:
		case <-ctx.Done():
			break sendLoop
		}
	}
	close(jobs)
	wg.Wait()
	select {
	case err := <-errs:
		return err
	default:
		return ctx.Err()
	}
}

func setOptional(properties Properties, key, value string) {
	if value != "" {
		properties[key] = value
	}
}

func cloneProperties(properties Properties) Properties {
	out := make(Properties, len(properties))
	for key, value := range properties {
		out[key] = value
	}
	return out
}
