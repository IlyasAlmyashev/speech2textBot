package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	telegramAPI      = "https://api.telegram.org"
	secretHeader     = "X-Telegram-Bot-Api-Secret-Token"
	messageNotEdited = "message is not modified"
)

type update struct {
	UpdateID int64    `json:"update_id"`
	Message  *message `json:"message"`
}

type message struct {
	MessageID     int64          `json:"message_id"`
	From          *user          `json:"from"`
	SenderChat    *chat          `json:"sender_chat"`
	Chat          chat           `json:"chat"`
	ForwardOrigin *messageOrigin `json:"forward_origin"`
	Text          string         `json:"text"`
	Voice         *media         `json:"voice"`
	Audio         *media         `json:"audio"`
	VideoNote     *media         `json:"video_note"`
}

type user struct {
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

type chat struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Username string `json:"username"`
}

// messageOrigin covers all four MessageOrigin variants; Type selects the field.
type messageOrigin struct {
	Type           string `json:"type"`
	SenderUser     *user  `json:"sender_user"`
	SenderUserName string `json:"sender_user_name"`
	SenderChat     *chat  `json:"sender_chat"`
	Chat           *chat  `json:"chat"`
}

// media is the part shared by Voice, Audio and VideoNote.
type media struct {
	FileID   string `json:"file_id"`
	FileSize int64  `json:"file_size"`
}

type telegramClient struct {
	baseURL string
	token   string
	http    *http.Client
	timeout time.Duration
	retry   retrier
}

// call invokes a Bot API method, retrying transient failures.
func (c *telegramClient) call(ctx context.Context, method string, payload, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode %s: %w", method, err)
	}
	err = c.retry.do(ctx, func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, c.timeout)
		defer cancel()
		return c.callOnce(ctx, method, body, out)
	})
	if err != nil {
		return fmt.Errorf("telegram %s: %w", method, err)
	}
	return nil
}

func (c *telegramClient) callOnce(ctx context.Context, method string, body []byte, out any) error {
	target := fmt.Sprintf("%s/bot%s/%s", c.baseURL, c.token, method)
	req, err := newRequest(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := send(c.http, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var envelope struct {
		OK          bool            `json:"ok"`
		Result      json.RawMessage `json:"result"`
		Description string          `json:"description"`
		Parameters  struct {
			RetryAfter int `json:"retry_after"`
		} `json:"parameters"`
	}
	decodeErr := json.NewDecoder(resp.Body).Decode(&envelope)
	if resp.StatusCode/100 != 2 || !envelope.OK {
		description := envelope.Description
		if description == "" {
			description = http.StatusText(resp.StatusCode)
		}
		return &apiError{
			Service:    "telegram",
			Status:     resp.StatusCode,
			Message:    description,
			RetryAfter: time.Duration(envelope.Parameters.RetryAfter) * time.Second,
		}
	}
	if decodeErr != nil {
		return fmt.Errorf("decode response: %w", decodeErr)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(envelope.Result, out)
}

func (c *telegramClient) setWebhook(ctx context.Context, webhookURL, secret string) error {
	return c.call(ctx, "setWebhook", map[string]any{
		"url":             webhookURL,
		"secret_token":    secret,
		"allowed_updates": []string{"message"},
	}, nil)
}

func (c *telegramClient) sendMessage(ctx context.Context, chatID int64, text string) (int64, error) {
	var sent struct {
		MessageID int64 `json:"message_id"`
	}
	err := c.call(ctx, "sendMessage", map[string]any{"chat_id": chatID, "text": text}, &sent)
	return sent.MessageID, err
}

// editMessage replaces a message's text. An edit that changes nothing counts as
// success: AssemblyAI may redeliver a webhook whose edit has already been applied.
func (c *telegramClient) editMessage(ctx context.Context, chatID, messageID int64, text string) error {
	err := c.call(ctx, "editMessageText", map[string]any{
		"chat_id":    chatID,
		"message_id": messageID,
		"text":       text,
	}, nil)
	if err != nil && strings.Contains(err.Error(), messageNotEdited) {
		return nil
	}
	return err
}

func (c *telegramClient) getFilePath(ctx context.Context, fileID string) (string, error) {
	var file struct {
		FilePath string `json:"file_path"`
	}
	if err := c.call(ctx, "getFile", map[string]any{"file_id": fileID}, &file); err != nil {
		return "", err
	}
	return file.FilePath, nil
}

// openFile starts a download. The caller closes the returned body.
func (c *telegramClient) openFile(ctx context.Context, filePath string) (io.ReadCloser, int64, error) {
	target := fmt.Sprintf("%s/file/bot%s/%s", c.baseURL, c.token, filePath)
	req, err := newRequest(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := send(c.http, req)
	if err != nil {
		return nil, 0, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, 0, &apiError{Service: "telegram file", Status: resp.StatusCode, Message: resp.Status}
	}
	return resp.Body, resp.ContentLength, nil
}
