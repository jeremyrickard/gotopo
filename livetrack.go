package gotopo

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type LiveTrackOptions struct {
	Title    string
	DeviceID string
	Width    float64
	Opacity  *float64
	Color    string
	Pattern  string
	FolderID string
	CreateOptions
}

func (c *Client) AddLiveTrack(ctx context.Context, opts LiveTrackOptions) (Feature, error) {
	if !strings.HasPrefix(opts.DeviceID, "FLEET:") {
		opts.DeviceID = "FLEET:" + opts.DeviceID
	}
	device := strings.TrimPrefix(opts.DeviceID, "FLEET:")
	if before, after, ok := strings.Cut(device, "-"); !ok || before == "" || after == "" {
		return Feature{}, fmt.Errorf("gotopo: live-track device ID must contain a non-empty group and device separated by a hyphen")
	}
	if opts.Title == "" {
		opts.Title = "New LiveTrack"
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
	properties := Properties{
		"title": opts.Title, "deviceId": opts.DeviceID,
		"stroke-width": opts.Width, "stroke-opacity": opacity,
		"stroke": opts.Color, "pattern": opts.Pattern,
	}
	setOptional(properties, "folderId", opts.FolderID)
	return c.addFeature(ctx, "LiveTrack", Feature{Properties: properties}, opts.Queue)
}

func (c *Client) UpdateLiveTrack(ctx context.Context, id string, latitude, longitude float64, elevation *float64) error {
	feature, err := c.GetFeature(ctx, FeatureFilter{ID: id, Class: "LiveTrack"})
	if err != nil {
		return err
	}
	device := feature.Properties.String("deviceId")
	if !strings.HasPrefix(device, "FLEET:") {
		return fmt.Errorf("gotopo: live-track device ID %q is malformed", device)
	}
	group, deviceID, ok := strings.Cut(strings.TrimPrefix(device, "FLEET:"), "-")
	if !ok || group == "" || deviceID == "" {
		return fmt.Errorf("gotopo: live-track device ID %q is malformed", device)
	}
	values := url.Values{
		"id": {deviceID}, "lat": {fmt.Sprint(latitude)}, "lng": {fmt.Sprint(longitude)},
	}
	if elevation != nil {
		values.Set("ele", fmt.Sprint(*elevation))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/position/report/"+url.PathEscape(group)+"?"+values.Encode(), nil)
	if err != nil {
		return fmt.Errorf("gotopo: create live-track update: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("gotopo: update live track: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return &APIError{Method: http.MethodGet, URL: req.URL.String(), StatusCode: resp.StatusCode}
	}
	return nil
}

func (c *Client) StopLiveTrack(ctx context.Context, id string) (Feature, error) {
	track, err := c.GetFeature(ctx, FeatureFilter{ID: id, Class: "LiveTrack", ForceRefresh: true})
	if err != nil {
		return Feature{}, err
	}
	if track.Geometry == nil {
		return Feature{}, fmt.Errorf("gotopo: live track %s has no geometry", id)
	}
	points, ok := positions(track.Geometry.Coordinates)
	if !ok {
		return Feature{}, fmt.Errorf("gotopo: live track %s has invalid line geometry", id)
	}
	opacity := numberPointer(track.Properties["stroke-opacity"])
	line, err := c.AddLine(ctx, LineOptions{
		Points: points, Title: track.Title(),
		Width: number(track.Properties["stroke-width"]), Opacity: opacity,
		Color: track.Properties.String("stroke"), Pattern: track.Properties.String("pattern"),
		FolderID: track.Properties.String("folderId"),
	})
	if err != nil {
		return Feature{}, err
	}
	if _, err := c.DeleteFeature(ctx, DeleteTarget{ID: id, Class: "LiveTrack"}); err != nil {
		return Feature{}, err
	}
	return line, nil
}

func number(value any) float64 {
	v, _ := value.(float64)
	return v
}

func numberPointer(value any) *float64 {
	v, ok := value.(float64)
	if !ok {
		return nil
	}
	return &v
}
