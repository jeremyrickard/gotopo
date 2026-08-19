package gotopo

import (
	"context"
	"fmt"
	"strings"
)

type OperationalPeriodOptions struct {
	Title         string
	Color         string
	StrokeOpacity *float64
	StrokeWidth   float64
	FillOpacity   *float64
	CreateOptions
}

func (c *Client) AddOperationalPeriod(ctx context.Context, opts OperationalPeriodOptions) (Feature, error) {
	if opts.Title == "" {
		opts.Title = "New OP"
	}
	if opts.Color == "" {
		opts.Color = "#FF0000"
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
	return c.addFeature(ctx, "OperationalPeriod", Feature{Properties: Properties{
		"title": opts.Title, "stroke": opts.Color, "fill": opts.Color,
		"stroke-opacity": strokeOpacity, "fill-opacity": fillOpacity,
		"stroke-width": opts.StrokeWidth,
	}}, opts.Queue)
}

type AssignmentOptions struct {
	Points              []Position
	Number              string
	Letter              string
	Title               string
	OperationalPeriodID string
	FolderID            string
	ResourceType        ResourceType
	TeamSize            int
	Priority            AssignmentPriority
	ResponsivePOD       AssignmentPriority
	UnresponsivePOD     AssignmentPriority
	CluePOD             AssignmentPriority
	Description         string
	PreviousEfforts     string
	Transportation      string
	TimeAllocated       any
	PrimaryFrequency    string
	SecondaryFrequency  string
	PreparedBy          string
	Status              AssignmentStatus
	CreateOptions
}

func (c *Client) AddAreaAssignment(ctx context.Context, opts AssignmentOptions) (Feature, error) {
	return c.addAssignment(ctx, opts, true)
}

func (c *Client) AddLineAssignment(ctx context.Context, opts AssignmentOptions) (Feature, error) {
	return c.addAssignment(ctx, opts, false)
}

func (c *Client) addAssignment(ctx context.Context, opts AssignmentOptions, area bool) (Feature, error) {
	minimum := 2
	if area {
		minimum = 3
	}
	if len(opts.Points) < minimum {
		return Feature{}, fmt.Errorf("gotopo: assignment requires at least %d points", minimum)
	}
	if opts.ResourceType == "" {
		opts.ResourceType = ResourceGround
	}
	if opts.Priority == "" {
		opts.Priority = PriorityLow
	}
	if opts.ResponsivePOD == "" {
		opts.ResponsivePOD = PriorityLow
	}
	if opts.UnresponsivePOD == "" {
		opts.UnresponsivePOD = PriorityLow
	}
	if opts.CluePOD == "" {
		opts.CluePOD = PriorityLow
	}
	if opts.Status == "" {
		opts.Status = AssignmentDraft
	}
	titleParts := make([]string, 0, 3)
	for _, part := range []string{opts.Title, opts.Letter, opts.Number} {
		if part != "" {
			titleParts = append(titleParts, part)
		}
	}
	properties := Properties{
		"title": strings.Join(titleParts, " "), "resourceType": string(opts.ResourceType),
		"teamSize": opts.TeamSize, "priority": string(opts.Priority),
		"responsivePOD":   string(opts.ResponsivePOD),
		"unresponsivePOD": string(opts.UnresponsivePOD), "cluePOD": string(opts.CluePOD),
		"description": opts.Description, "previousEfforts": opts.PreviousEfforts,
		"transportation": opts.Transportation, "timeAllocated": opts.TimeAllocated,
		"primaryFrequency": opts.PrimaryFrequency, "secondaryFrequency": opts.SecondaryFrequency,
		"preparedBy": opts.PreparedBy, "status": string(opts.Status),
	}
	setOptional(properties, "number", opts.Number)
	setOptional(properties, "letter", opts.Letter)
	setOptional(properties, "operationalPeriodId", opts.OperationalPeriodID)
	setOptional(properties, "folderId", opts.FolderID)
	geometry := &Geometry{Type: "LineString", Coordinates: opts.Points}
	if area {
		geometry = &Geometry{Type: "Polygon", Coordinates: [][]Position{opts.Points}}
	}
	return c.addFeature(ctx, "Assignment", Feature{Properties: properties, Geometry: geometry}, opts.Queue)
}
