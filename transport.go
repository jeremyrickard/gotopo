package gotopo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type requestSpec struct {
	method    string
	path      string
	payload   any
	accountID string
}

func (c *Client) mapPath(endpoint, id string) (string, error) {
	mapID, err := c.requireMap()
	if err != nil {
		return "", err
	}
	if strings.Contains(strings.ToLower(endpoint), "api/") {
		if strings.HasPrefix(endpoint, "/") {
			return endpointWithID(endpoint, id), nil
		}
		return endpointWithID("/"+endpoint, id), nil
	}
	if endpoint == strings.ToLower(endpoint) && endpoint != "since" && !strings.HasPrefix(endpoint, "since/") {
		endpoint = strings.ToUpper(endpoint[:1]) + endpoint[1:]
	}
	prefix := "/"
	if c.hosted {
		prefix = "/api/v1/map/" + mapID + "/"
	}
	return endpointWithID(prefix+endpoint, id), nil
}

func endpointWithID(path, id string) string {
	if id == "" {
		return path
	}
	return strings.TrimRight(path, "/") + "/" + id
}

func (c *Client) do(ctx context.Context, spec requestSpec, out any) error {
	if ctx == nil {
		return fmt.Errorf("gotopo: nil context")
	}
	method := strings.ToUpper(spec.method)
	var payload []byte
	var err error
	if spec.payload != nil {
		payload, err = json.Marshal(spec.payload)
		if err != nil {
			return fmt.Errorf("gotopo: encode request payload: %w", err)
		}
	}

	values := url.Values{}
	if c.hosted {
		expires := c.clock().Add(2 * time.Minute).UnixMilli()
		signature, err := signRequest(c.credentials.Key, method, spec.path, expires, payload)
		if err != nil {
			return err
		}
		values.Set("id", c.credentials.ID)
		values.Set("expires", fmt.Sprint(expires))
		values.Set("signature", signature)
		if method != http.MethodPost {
			values.Set("json", "")
		}
	}
	if method == http.MethodPost {
		values.Set("json", string(payload))
	} else if !c.hosted && spec.payload != nil {
		for k, v := range payloadQueryValues(spec.payload) {
			values.Set(k, v)
		}
	}

	requestURL := c.baseURL + spec.path
	var body io.Reader
	if method == http.MethodPost {
		body = strings.NewReader(values.Encode())
	} else if len(values) != 0 {
		requestURL += "?" + values.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL, body)
	if err != nil {
		return fmt.Errorf("gotopo: create request: %w", err)
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("gotopo: %s %s: %w", method, requestURL, err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return fmt.Errorf("gotopo: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return &APIError{Method: method, URL: requestURL, StatusCode: resp.StatusCode, Body: string(responseBody)}
	}
	if out == nil || len(bytes.TrimSpace(responseBody)) == 0 {
		return nil
	}
	var envelope responseEnvelope
	if err := json.Unmarshal(responseBody, &envelope); err != nil {
		return fmt.Errorf("gotopo: decode response: %w", err)
	}
	if envelope.Status != "" && !strings.EqualFold(envelope.Status, "ok") {
		return &APIError{Method: method, URL: requestURL, StatusCode: resp.StatusCode, Status: envelope.Status, Message: envelope.Message, Body: string(responseBody)}
	}
	if raw, ok := out.(*json.RawMessage); ok {
		*raw = append((*raw)[:0], envelope.Result...)
		return nil
	}
	if len(envelope.Result) == 0 || string(envelope.Result) == "null" {
		return nil
	}
	if err := json.Unmarshal(envelope.Result, out); err != nil {
		return fmt.Errorf("gotopo: decode response result: %w", err)
	}
	return nil
}

func payloadQueryValues(payload any) map[string]string {
	out := make(map[string]string)
	if m, ok := payload.(map[string]string); ok {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}
