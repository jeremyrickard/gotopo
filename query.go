package gotopo

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// FeatureFilter controls local cache queries.
type FeatureFilter struct {
	Class               string
	Title               string
	ID                  string
	ExcludeClasses      []string
	LetterOnly          bool
	AllowMultipleTitles bool
	ForceRefresh        bool
}

// GetFeatures returns cloned features matching a cache filter.
func (c *Client) GetFeatures(ctx context.Context, filter FeatureFilter) ([]Feature, error) {
	if err := c.Refresh(ctx, filter.ForceRefresh); err != nil {
		return nil, err
	}
	excluded := make(map[string]struct{}, len(filter.ExcludeClasses))
	for _, class := range filter.ExcludeClasses {
		excluded[class] = struct{}{}
	}
	c.mu.RLock()
	matches := make([]Feature, 0)
	titleMatches := 0
	for _, feature := range c.features {
		if filter.ID != "" {
			if feature.ID != filter.ID || (filter.Class != "" && !c.equal(feature.Class(), filter.Class)) {
				continue
			}
			matches = append(matches, feature)
			continue
		}
		if filter.Class != "" && !c.equal(feature.Class(), filter.Class) {
			continue
		}
		if filter.Class == "" {
			if _, skip := excluded[feature.Class()]; skip {
				continue
			}
		}
		if filter.Title == "" {
			matches = append(matches, feature)
			continue
		}
		title := strings.TrimSpace(feature.Title())
		match := c.equal(title, filter.Title)
		if filter.LetterOnly {
			fields := strings.Fields(title)
			match = len(fields) > 0 && c.equal(fields[0], filter.Title)
		}
		if !match {
			match = c.equal(strings.TrimSpace(feature.Properties.String("letter")), filter.Title)
		}
		if match {
			titleMatches++
			matches = append(matches, feature)
		}
	}
	c.mu.RUnlock()
	if titleMatches > 1 && !filter.AllowMultipleTitles {
		return nil, ErrAmbiguousMatch
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Class() == matches[j].Class() {
			return matches[i].ID < matches[j].ID
		}
		return matches[i].Class() < matches[j].Class()
	})
	out := make([]Feature, 0, len(matches))
	for _, feature := range matches {
		clone, err := feature.Clone()
		if err != nil {
			return nil, err
		}
		out = append(out, clone)
	}
	return out, nil
}

// GetFeature returns exactly one matching feature.
func (c *Client) GetFeature(ctx context.Context, filter FeatureFilter) (Feature, error) {
	features, err := c.GetFeatures(ctx, filter)
	if err != nil {
		return Feature{}, err
	}
	if len(features) == 0 {
		return Feature{}, ErrNotFound
	}
	if len(features) != 1 {
		return Feature{}, fmt.Errorf("%w: found %d", ErrAmbiguousMatch, len(features))
	}
	return features[0], nil
}

func (c *Client) equal(a, b string) bool {
	if c.caseSensitive {
		return a == b
	}
	return strings.EqualFold(a, b)
}
