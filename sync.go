package gotopo

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Refresh updates the local cache when it is stale, or immediately when force
// is true.
func (c *Client) Refresh(ctx context.Context, force bool) error {
	if _, err := c.requireMap(); err != nil {
		return err
	}
	c.mu.RLock()
	stale := c.lastLocalSync.IsZero() || c.clock().Sub(c.lastLocalSync) > c.syncInterval
	since := c.lastServerSync
	c.mu.RUnlock()
	if !force && !stale {
		return nil
	}
	if since > 500 {
		since -= 500
	} else {
		since = 0
	}
	path, err := c.mapPath(fmt.Sprintf("since/%d", since), "")
	if err != nil {
		return err
	}
	var result syncResult
	err = c.do(ctx, requestSpec{method: http.MethodGet, path: path}, &result)
	if err != nil {
		c.connectionFailed(err)
		return err
	}
	c.applySync(result)
	return nil
}

// StartSync starts one background synchronization loop.
func (c *Client) StartSync(parent context.Context) error {
	if _, err := c.requireMap(); err != nil {
		return err
	}
	c.mu.Lock()
	if c.syncCancel != nil {
		c.mu.Unlock()
		return nil
	}
	ctx, cancel := context.WithCancel(parent)
	c.syncCancel = cancel
	c.syncWG.Add(1)
	c.mu.Unlock()
	go c.syncLoop(ctx)
	return nil
}

func (c *Client) syncLoop(ctx context.Context) {
	defer c.syncWG.Done()
	ticker := time.NewTicker(c.syncInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refreshCtx, cancel := context.WithTimeout(ctx, c.requestTimeout)
			_ = c.Refresh(refreshCtx, true)
			cancel()
		}
	}
}

// StopSync stops background synchronization and waits for it to exit.
func (c *Client) StopSync() {
	c.mu.Lock()
	cancel := c.syncCancel
	c.syncCancel = nil
	c.mu.Unlock()
	if cancel != nil {
		cancel()
		c.syncWG.Wait()
	}
}

func (c *Client) connectionFailed(err error) {
	c.mu.Lock()
	first := !c.disconnected
	c.disconnected = true
	handler := c.events.Disconnected
	c.mu.Unlock()
	if first && handler != nil {
		handler(err)
	}
}

func (c *Client) applySync(result syncResult) {
	var added, updated []Feature
	var deleted []DeleteTarget
	c.mu.Lock()
	wasDisconnected := c.disconnected
	c.disconnected = false

	if result.IDs != nil {
		c.ids = cloneIDs(result.IDs)
		allowed := make(map[string]struct{})
		for class, ids := range result.IDs {
			for _, id := range ids {
				allowed[featureKey(class, id)] = struct{}{}
			}
		}
		for key, feature := range c.features {
			if _, ok := allowed[key]; !ok {
				delete(c.features, key)
				deleted = append(deleted, DeleteTarget{ID: feature.ID, Class: feature.Class()})
			}
		}
	}

	for _, incoming := range result.State.Features {
		normalizeAssignmentTitle(&incoming)
		key := featureKey(incoming.Class(), incoming.ID)
		existing, ok := c.features[key]
		if !ok {
			c.features[key] = incoming
			c.addIDLocked(incoming.Class(), incoming.ID)
			if eventFeature, err := incoming.Clone(); err == nil {
				added = append(added, eventFeature)
			}
			continue
		}
		merged := mergeFeature(existing, incoming)
		if !featuresEqual(existing, merged) {
			c.features[key] = merged
			if eventFeature, err := merged.Clone(); err == nil {
				updated = append(updated, eventFeature)
			}
		}
	}
	c.lastServerSync = result.Timestamp
	c.lastLocalSync = c.clock()
	handlers := c.events
	c.mu.Unlock()

	if wasDisconnected && handlers.Reconnected != nil {
		handlers.Reconnected()
	}
	for _, feature := range added {
		if handlers.FeatureAdded != nil {
			handlers.FeatureAdded(feature)
		}
	}
	for _, feature := range updated {
		if handlers.FeatureUpdated != nil {
			handlers.FeatureUpdated(feature)
		}
	}
	for _, target := range deleted {
		if handlers.FeatureDeleted != nil {
			handlers.FeatureDeleted(target.ID, target.Class)
		}
	}
	if handlers.Synced != nil {
		handlers.Synced()
	}
}

func cloneIDs(ids map[string][]string) map[string][]string {
	out := make(map[string][]string, len(ids))
	for class, values := range ids {
		out[class] = append([]string(nil), values...)
	}
	return out
}
