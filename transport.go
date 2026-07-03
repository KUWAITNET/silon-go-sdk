package silon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	mrand "math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxRetryDelaySeconds = 30.0

var retryableStatuses = map[int]bool{
	http.StatusTooManyRequests:     true, // 429
	http.StatusInternalServerError: true, // 500
	http.StatusBadGateway:          true, // 502
	http.StatusServiceUnavailable:  true, // 503
	http.StatusGatewayTimeout:      true, // 504
}

var idempotentMethods = map[string]bool{
	http.MethodGet:     true,
	http.MethodHead:    true,
	http.MethodOptions: true,
	http.MethodPut:     true,
	http.MethodDelete:  true,
}

// requestSpec describes one logical API call, replayable across retries.
type requestSpec struct {
	method  string
	path    string            // e.g. "/api/v1/messages/"
	query   url.Values        // optional
	body    any               // JSON-marshalled once when non-nil
	rawBody []byte            // pre-encoded body (e.g. multipart); wins over body
	rawType string            // Content-Type for rawBody
	headers map[string]string // per-call extras (e.g. Idempotency-Key)
}

// do executes spec against the configured base URL, retrying per the SPEC 3
// gating rules, and decodes a JSON success body into out (which may be nil
// to discard). A 204 or empty body leaves out untouched and returns nil.
func (c *Client) do(ctx context.Context, spec requestSpec, out any) error {
	headers := c.buildHeaders(spec.headers)

	var bodyBytes []byte
	switch {
	case spec.rawBody != nil:
		bodyBytes = spec.rawBody
		headers.Set("Content-Type", spec.rawType)
	case spec.body != nil:
		encoded, err := json.Marshal(spec.body)
		if err != nil {
			return &Error{Message: "Could not encode request body as JSON: " + err.Error()}
		}
		bodyBytes = encoded
		headers.Set("Content-Type", "application/json")
	}

	requestURL := c.baseURL + spec.path
	if len(spec.query) > 0 {
		requestURL += "?" + spec.query.Encode()
	}
	hasIdempotencyKey := headers.Get("Idempotency-Key") != ""

	attempt := 0
	for {
		var reader io.Reader
		if bodyBytes != nil {
			reader = bytes.NewReader(bodyBytes)
		}
		req, err := http.NewRequestWithContext(ctx, spec.method, requestURL, reader)
		if err != nil {
			return &Error{Message: "Could not build request: " + err.Error()}
		}
		req.Header = headers.Clone()

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if c.shouldRetry(spec.method, hasIdempotencyKey, attempt, 0) {
				if sleepErr := c.sleep(ctx, c.retryDelay(nil, attempt)); sleepErr != nil {
					return sleepErr
				}
				attempt++
				continue
			}
			return newConnectionError(spec.path, err)
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			if c.shouldRetry(spec.method, hasIdempotencyKey, attempt, 0) {
				if sleepErr := c.sleep(ctx, c.retryDelay(nil, attempt)); sleepErr != nil {
					return sleepErr
				}
				attempt++
				continue
			}
			return newConnectionError(spec.path, readErr)
		}

		if resp.StatusCode >= 400 {
			if c.shouldRetry(spec.method, hasIdempotencyKey, attempt, resp.StatusCode) {
				if sleepErr := c.sleep(ctx, c.retryDelay(resp.Header, attempt)); sleepErr != nil {
					return sleepErr
				}
				attempt++
				continue
			}
			return newAPIError(resp, respBody)
		}

		// 204 / empty body -> void result.
		if resp.StatusCode == http.StatusNoContent || len(respBody) == 0 {
			return nil
		}
		if out == nil {
			if !json.Valid(respBody) {
				return &Error{Message: fmt.Sprintf(
					"Could not parse response body as JSON (HTTP %d).", resp.StatusCode)}
			}
			return nil
		}
		if err := json.Unmarshal(respBody, out); err != nil {
			return &Error{Message: fmt.Sprintf(
				"Could not parse response body as JSON (HTTP %d).", resp.StatusCode)}
		}
		return nil
	}
}

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	return c.do(ctx, requestSpec{method: http.MethodGet, path: path, query: query}, out)
}

func (c *Client) post(ctx context.Context, path string, body any, headers map[string]string, out any) error {
	return c.do(ctx, requestSpec{method: http.MethodPost, path: path, body: body, headers: headers}, out)
}

func (c *Client) patch(ctx context.Context, path string, body any, out any) error {
	return c.do(ctx, requestSpec{method: http.MethodPatch, path: path, body: body}, out)
}

func (c *Client) put(ctx context.Context, path string, body any, out any) error {
	return c.do(ctx, requestSpec{method: http.MethodPut, path: path, body: body}, out)
}

func (c *Client) delete(ctx context.Context, path string) error {
	return c.do(ctx, requestSpec{method: http.MethodDelete, path: path}, nil)
}

// buildHeaders layers, in order: standard auth/UA headers, the client's
// default headers, then per-call extras (later layers win).
func (c *Client) buildHeaders(extra map[string]string) http.Header {
	h := http.Header{}
	h.Set("Authorization", "Bearer "+c.apiKey)
	h.Set("Accept", "application/json")
	h.Set("User-Agent", userAgent())
	for k, v := range c.defaultHeaders {
		h.Set(k, v)
	}
	for k, v := range extra {
		h.Set(k, v)
	}
	return h
}

// shouldRetry reports whether another attempt may be made. status == 0
// means the request produced no HTTP response (connection error/timeout).
// POST/PATCH are only replayed when the request carries an
// Idempotency-Key, so a retry can never double-send.
func (c *Client) shouldRetry(method string, hasIdempotencyKey bool, attempt, status int) bool {
	if attempt >= c.maxRetries {
		return false
	}
	if !idempotentMethods[strings.ToUpper(method)] && !hasIdempotencyKey {
		return false
	}
	return status == 0 || retryableStatuses[status]
}

// retryDelay computes min(0.5 * 2^attempt, 8) + jitter(0..0.25) seconds,
// raised to the server's Retry-After / RateLimit-Reset hint when that is
// larger, clamped to [0, 30] seconds.
func (c *Client) retryDelay(respHeaders http.Header, attempt int) time.Duration {
	delay := math.Min(0.5*math.Exp2(float64(attempt)), 8.0) + mrand.Float64()*0.25
	if respHeaders != nil {
		if advertised := parseRetryAfter(respHeaders); advertised != nil && *advertised > delay {
			delay = *advertised
		}
	}
	delay = math.Max(0.0, math.Min(delay, maxRetryDelaySeconds))
	return time.Duration(delay * float64(time.Second))
}

// parseRetryAfter returns the seconds the server asked us to wait before
// retrying, if advertised. It reads the standard Retry-After header
// (delta-seconds or HTTP-date) and falls back to the IETF draft
// RateLimit-Reset header, which Silon sends as a Unix epoch on throttled
// endpoints.
func parseRetryAfter(h http.Header) *float64 {
	if retryAfter := h.Get("Retry-After"); retryAfter != "" {
		if v, err := strconv.ParseFloat(retryAfter, 64); err == nil {
			v = math.Max(0.0, v)
			return &v
		}
		if when, err := http.ParseTime(retryAfter); err == nil {
			v := math.Max(0.0, time.Until(when).Seconds())
			return &v
		}
		return nil
	}
	if reset := h.Get("RateLimit-Reset"); reset != "" {
		if epoch, err := strconv.ParseFloat(reset, 64); err == nil {
			now := float64(time.Now().UnixMilli()) / 1000.0
			v := math.Max(0.0, epoch-now)
			return &v
		}
		return nil
	}
	return nil
}

func newConnectionError(path string, err error) *ConnectionError {
	if isTimeout(err) {
		return &ConnectionError{Timeout: true, msg: "Request timed out.", err: err}
	}
	return &ConnectionError{
		msg: fmt.Sprintf("Connection error while requesting %s: %v", path, err),
		err: err,
	}
}

func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
