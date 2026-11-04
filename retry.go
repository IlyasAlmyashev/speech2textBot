package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// apiError is a non-2xx answer from an external service.
type apiError struct {
	Service    string
	Status     int
	Message    string
	RetryAfter time.Duration
}

func (e *apiError) Error() string {
	return fmt.Sprintf("%s: status %d: %s", e.Service, e.Status, e.Message)
}

// transportError is a failure to get any answer at all. It deliberately keeps
// only the host: net/http errors quote the full URL, and Telegram URLs contain
// the bot token, which must never reach the logs.
type transportError struct {
	Method string
	Host   string
	Err    error
}

func (e *transportError) Error() string {
	return fmt.Sprintf("%s %s: %v", e.Method, e.Host, e.Err)
}

func (e *transportError) Unwrap() error { return e.Err }

// newRequest wraps http.NewRequestWithContext, dropping the URL from parse errors.
func newRequest(ctx context.Context, method, target string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, fmt.Errorf("build %s request: %w", method, withoutURL(err))
	}
	return req, nil
}

// send performs req and converts failures into a transportError.
func send(client *http.Client, req *http.Request) (*http.Response, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, &transportError{Method: req.Method, Host: req.URL.Host, Err: withoutURL(err)}
	}
	return resp, nil
}

func withoutURL(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err
	}
	return err
}

// isRetryable reports whether repeating the failed operation could help.
func isRetryable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	var api *apiError
	if errors.As(err, &api) {
		return api.Status == http.StatusTooManyRequests || api.Status >= http.StatusInternalServerError
	}
	var transport *transportError
	return errors.As(err, &transport)
}

// retrier repeats retryable operations a fixed number of times.
type retrier struct {
	attempts int
	delay    time.Duration
}

func (r retrier) do(ctx context.Context, op func(context.Context) error) error {
	for attempt := 1; ; attempt++ {
		err := op(ctx)
		if err == nil || !isRetryable(err) || attempt >= r.attempts {
			return err
		}

		wait := r.delay
		var api *apiError
		if errors.As(err, &api) && api.RetryAfter > wait {
			wait = api.RetryAfter
		}

		timer := time.NewTimer(wait)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return err
		}
	}
}
