package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	assemblyAPI          = "https://api.assemblyai.com"
	assemblyAuthHeader   = "X-Webhook-Secret"
	transcriptCompleted  = "completed"
	assemblyErrorBodyMax = 4 << 10
)

type transcript struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Text   string `json:"text"`
	Error  string `json:"error"`
}

type assemblyClient struct {
	baseURL string
	key     string
	secret  string
	models  []string
	http    *http.Client
	timeout time.Duration
	retry   retrier
}

// do sends one request and decodes the JSON answer into out.
func (c *assemblyClient) do(req *http.Request, out any) error {
	req.Header.Set("Authorization", c.key)

	resp, err := send(c.http, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, assemblyErrorBodyMax))
		var body struct {
			Error string `json:"error"`
		}
		message := resp.Status
		if json.Unmarshal(raw, &body) == nil && body.Error != "" {
			message = body.Error
		}
		return &apiError{Service: "assemblyai", Status: resp.StatusCode, Message: message}
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// upload streams audio to AssemblyAI's private storage. A stream can only be
// consumed once, so the caller owns retries. size is -1 when it is unknown.
func (c *assemblyClient) upload(ctx context.Context, audio io.Reader, size int64) (string, error) {
	req, err := newRequest(ctx, http.MethodPost, c.baseURL+"/v2/upload", audio)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = size

	var uploaded struct {
		UploadURL string `json:"upload_url"`
	}
	if err := c.do(req, &uploaded); err != nil {
		return "", fmt.Errorf("assemblyai upload: %w", err)
	}
	return uploaded.UploadURL, nil
}

// submit queues a transcription. AssemblyAI calls webhookURL when it finishes.
func (c *assemblyClient) submit(ctx context.Context, audioURL, webhookURL string) (string, error) {
	body, err := json.Marshal(map[string]any{
		"audio_url":                 audioURL,
		"speech_models":             c.models,
		"language_detection":        true,
		"webhook_url":               webhookURL,
		"webhook_auth_header_name":  assemblyAuthHeader,
		"webhook_auth_header_value": c.secret,
	})
	if err != nil {
		return "", fmt.Errorf("encode transcript request: %w", err)
	}

	var queued transcript
	err = c.retry.do(ctx, func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, c.timeout)
		defer cancel()
		req, err := newRequest(ctx, http.MethodPost, c.baseURL+"/v2/transcript", bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		return c.do(req, &queued)
	})
	if err != nil {
		return "", fmt.Errorf("assemblyai submit: %w", err)
	}
	return queued.ID, nil
}

func (c *assemblyClient) transcript(ctx context.Context, id string) (transcript, error) {
	var result transcript
	err := c.retry.do(ctx, func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, c.timeout)
		defer cancel()
		req, err := newRequest(ctx, http.MethodGet, c.baseURL+"/v2/transcript/"+id, nil)
		if err != nil {
			return err
		}
		return c.do(req, &result)
	})
	if err != nil {
		return transcript{}, fmt.Errorf("assemblyai transcript %s: %w", id, err)
	}
	return result, nil
}
