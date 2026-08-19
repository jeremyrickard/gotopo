package gotopo

import (
	"encoding/json"
	"reflect"
)

func normalizeAssignmentTitle(feature *Feature) {
	if feature.Class() != "Assignment" {
		return
	}
	letter := feature.Properties.String("letter")
	number := feature.Properties.String("number")
	if letter != "" || number != "" {
		title := letter
		if title != "" && number != "" {
			title += " "
		}
		title += number
		feature.Properties["title"] = title
	}
}

func mergeFeature(existing, incoming Feature) Feature {
	out := existing
	if incoming.Type != "" {
		out.Type = incoming.Type
	}
	if incoming.Properties != nil {
		if _, full := incoming.Properties["title"]; full {
			out.Properties = incoming.Properties
		}
	}
	if incoming.Geometry != nil {
		if existing.Geometry != nil && incoming.Geometry.Incremental {
			if oldLine, ok := positions(existing.Geometry.Coordinates); ok {
				if newLine, ok := positions(incoming.Geometry.Coordinates); ok {
					latest := timestampOfLast(oldLine)
					for i, point := range newLine {
						if pointTimestamp(point) > latest {
							oldLine = append(oldLine, newLine[i:]...)
							break
						}
					}
					geom := *existing.Geometry
					geom.Coordinates = oldLine
					geom.Size = len(oldLine)
					out.Geometry = &geom
					return out
				}
			}
		}
		out.Geometry = incoming.Geometry
	}
	return out
}

func positions(value any) ([]Position, bool) {
	if points, ok := value.([]Position); ok {
		return append([]Position(nil), points...), true
	}
	b, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	var points []Position
	if err := json.Unmarshal(b, &points); err != nil {
		return nil, false
	}
	return points, true
}

func polygonPositions(value any) ([][]Position, bool) {
	if rings, ok := value.([][]Position); ok {
		return rings, true
	}
	b, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	var rings [][]Position
	if err := json.Unmarshal(b, &rings); err != nil {
		return nil, false
	}
	return rings, true
}

func pointPosition(value any) (Position, bool) {
	if point, ok := value.(Position); ok {
		return append(Position(nil), point...), true
	}
	b, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	var point Position
	if err := json.Unmarshal(b, &point); err != nil {
		return nil, false
	}
	return point, true
}

func timestampOfLast(points []Position) float64 {
	if len(points) == 0 {
		return -1
	}
	return pointTimestamp(points[len(points)-1])
}

func pointTimestamp(point Position) float64 {
	if len(point) > 3 {
		return point[3]
	}
	return -1
}

func featuresEqual(a, b Feature) bool { return reflect.DeepEqual(a, b) }

func (c *Client) addIDLocked(class, id string) {
	for _, existing := range c.ids[class] {
		if existing == id {
			return
		}
	}
	c.ids[class] = append(c.ids[class], id)
}

func (c *Client) upsertFeature(feature Feature) {
	eventFeature, cloneErr := feature.Clone()
	c.mu.Lock()
	key := featureKey(feature.Class(), feature.ID)
	_, existed := c.features[key]
	c.features[key] = feature
	c.addIDLocked(feature.Class(), feature.ID)
	handler := c.events.FeatureAdded
	if existed {
		handler = c.events.FeatureUpdated
	}
	c.mu.Unlock()
	if handler != nil && cloneErr == nil {
		handler(eventFeature)
	}
}

func (c *Client) removeFeature(id, class string) {
	c.mu.Lock()
	delete(c.features, featureKey(class, id))
	values := c.ids[class]
	for i, value := range values {
		if value == id {
			c.ids[class] = append(values[:i], values[i+1:]...)
			break
		}
	}
	handler := c.events.FeatureDeleted
	c.mu.Unlock()
	if handler != nil {
		handler(id, class)
	}
}
