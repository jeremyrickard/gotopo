package gotopo

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Client is a concurrency-safe CalTopo session and local map cache.
type Client struct {
	baseURL         string
	hosted          bool
	httpClient      httpDoer
	credentials     Credentials
	syncInterval    time.Duration
	requestTimeout  time.Duration
	caseSensitive   bool
	pointValidation PointValidation
	clock           func() time.Time
	events          EventHandlers

	mu             sync.RWMutex
	mapID          string
	features       map[string]Feature
	ids            map[string][]string
	lastServerSync int64
	lastLocalSync  time.Time
	accountData    *AccountData
	queued         map[string][]Feature
	closed         bool
	disconnected   bool
	syncCancel     context.CancelFunc
	syncWG         sync.WaitGroup
}

func featureKey(class, id string) string { return class + "\x00" + id }

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// NewClient creates a mapless client. Use OpenMap to populate its cache.
func NewClient(opts ...Option) (*Client, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}
	host := strings.ToLower(cfg.baseURL.Hostname())
	hosted := host == "caltopo.com" || host == "sartopo.com" || host == "testing.caltopo.com"
	if hosted {
		if cfg.credentials.ID == "" || cfg.credentials.Key == "" {
			return nil, fmt.Errorf("gotopo: credential ID and key are required for %s", host)
		}
		if _, err := base64.StdEncoding.DecodeString(cfg.credentials.Key); err != nil {
			return nil, fmt.Errorf("gotopo: credential key is not valid base64: %w", err)
		}
	}
	return &Client{
		baseURL: cfg.baseURL.String(), hosted: hosted, httpClient: cfg.httpClient,
		credentials: cfg.credentials, syncInterval: cfg.syncInterval,
		requestTimeout: cfg.requestTimeout, caseSensitive: cfg.caseSensitive,
		pointValidation: cfg.pointValidation, clock: cfg.clock, events: cfg.events,
		features: make(map[string]Feature), ids: make(map[string][]string),
		queued: make(map[string][]Feature),
	}, nil
}

// MapID returns the currently open map ID.
func (c *Client) MapID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.mapID
}

// Close stops background synchronization and permanently closes the client.
func (c *Client) Close() error {
	c.StopSync()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	c.mapID = ""
	c.features = make(map[string]Feature)
	c.ids = make(map[string][]string)
	c.queued = make(map[string][]Feature)
	return nil
}

func (c *Client) requireMap() (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return "", ErrClosed
	}
	if c.mapID == "" {
		return "", ErrNoMap
	}
	return c.mapID, nil
}
