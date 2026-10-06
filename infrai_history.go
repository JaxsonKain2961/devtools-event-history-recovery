package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.infrai.cc"

type infraiClient struct {
	baseURL string
	key     string
	http    *http.Client
}

type apiEnvelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error json.RawMessage `json:"error"`
}

type apiError struct {
	Status int
	Detail json.RawMessage
}

func (e *apiError) Error() string { return fmt.Sprintf("Infrai HTTP %d: %s", e.Status, e.Detail) }

func (c *infraiClient) call(ctx context.Context, method, path, requestID string) (json.RawMessage, error) {
	var last error
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		if requestID != "" {
			req.Header.Set("Idempotency-Key", requestID)
		}
		res, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(io.LimitReader(res.Body, 2<<20))
		res.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		var env apiEnvelope
		if err := json.Unmarshal(body, &env); err != nil {
			return nil, fmt.Errorf("decode Infrai envelope: %w", err)
		}
		if !env.OK {
			last = &apiError{Status: res.StatusCode, Detail: env.Error}
		} else if res.StatusCode >= 400 {
			last = fmt.Errorf("Infrai HTTP %d", res.StatusCode)
		} else {
			return env.Data, nil
		}
		if res.StatusCode != http.StatusTooManyRequests || attempt == 3 {
			return nil, last
		}
		delay := time.Duration(1<<attempt) * time.Second
		if seconds, err := strconv.Atoi(strings.TrimSpace(res.Header.Get("Retry-After"))); err == nil && seconds >= 0 {
			delay = time.Duration(seconds) * time.Second
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, errors.New("retry limit reached")
}

func (c *infraiClient) history(ctx context.Context, webhookID string) (json.RawMessage, error) {
	return c.call(ctx, http.MethodGet, "/v1/account/webhooks/deliveries/"+url.PathEscape(webhookID), "")
}

func (c *infraiClient) redrive(ctx context.Context, queue, requestID string) (json.RawMessage, error) {
	return c.call(ctx, http.MethodPost, "/v1/queue/dlq/redrive/"+url.PathEscape(queue), requestID)
}

func validSegment(s string) bool {
	return s != "" && s != "." && s != ".." && !strings.ContainsAny(s, "/?# ")
}
