package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"runtime/debug"
	"strconv"
	"time"
	"unicode/utf16"
)

const (
	waitMessage     = "Your request has been accepted for processing, please wait."
	errorMessage    = "An error occurred, please try again later"
	noSpeechMessage = "No speech was found in this message."
	tooBigMessage   = "This file is too big: bots can only download files up to 20 MB."
	saidTemplate    = "%s said: \n\n%s"
	unknownAuthor   = "Someone"
	ellipsis        = "..."

	updatePath     = "/callback/update"
	assemblyPath   = "/callback/assembly"
	queryChatID    = "chat_id"
	queryMessageID = "message_id"
	queryAuthor    = "author"

	maxMessageLength = 4096
	maxTelegramFile  = 20 << 20
	maxWebhookBody   = 1 << 20
	assemblyDeadline = 9 * time.Second // AssemblyAI waits 10s for our answer
)

type bot struct {
	cfg      Config
	log      *slog.Logger
	telegram *telegramClient
	assembly *assemblyClient
}

func newBot(cfg Config, log *slog.Logger) *bot {
	client := &http.Client{} // per-operation timeouts come from contexts
	retry := retrier{attempts: cfg.RetryAttempts, delay: cfg.RetryDelay}
	return &bot{
		cfg: cfg,
		log: log,
		telegram: &telegramClient{
			baseURL: telegramAPI,
			token:   cfg.BotToken,
			http:    client,
			timeout: cfg.TelegramTimeout,
			retry:   retry,
		},
		assembly: &assemblyClient{
			baseURL: assemblyAPI,
			key:     cfg.AssemblyKey,
			secret:  cfg.WebhookSecret,
			models:  cfg.SpeechModels,
			http:    client,
			timeout: cfg.AssemblyTimeout,
			retry:   retry,
		},
	}
}

func (b *bot) routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST "+updatePath, b.endpoint("telegram update", b.handleUpdate))
	mux.Handle("POST "+assemblyPath, b.endpoint("assemblyai webhook", b.handleTranscript))
	return mux
}

// endpoint adapts a handler and is the single place where failures are logged.
// The deferred block covers panics too, and the status stays 500 in that case
// because the assignment below never completes.
func (b *bot) endpoint(op string, handler func(*http.Request) (int, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := http.StatusInternalServerError
		var err error

		defer func() {
			if p := recover(); p != nil {
				b.log.Error("panic in handler", "op", op, "panic", p, "stack", string(debug.Stack()))
			}
			if err != nil {
				b.log.Error("request failed", "op", op, "status", status, "error", err)
			}
			w.WriteHeader(status)
		}()

		status, err = handler(r)
	})
}

func (b *bot) authorized(given string) bool {
	return subtle.ConstantTimeCompare([]byte(given), []byte(b.cfg.WebhookSecret)) == 1
}

// handleUpdate acknowledges every authenticated update with 200: a non-2xx
// status makes Telegram redeliver it, which would only produce duplicates.
//
// The work stays inside the request on purpose. Cloud Run takes the CPU away
// once the response is sent, so a goroutine started here could stall.
func (b *bot) handleUpdate(r *http.Request) (int, error) {
	if !b.authorized(r.Header.Get(secretHeader)) {
		b.log.Warn("rejected telegram update: bad secret")
		return http.StatusUnauthorized, nil
	}

	var upd update
	body := http.MaxBytesReader(nil, r.Body, maxWebhookBody)
	if err := json.NewDecoder(body).Decode(&upd); err != nil {
		return http.StatusOK, fmt.Errorf("decode update: %w", err)
	}
	if upd.Message == nil {
		return http.StatusOK, nil
	}

	// A dropped connection must not abort work that is already half done.
	ctx := context.WithoutCancel(r.Context())
	if err := b.processMessage(ctx, upd.Message); err != nil {
		return http.StatusOK, fmt.Errorf("update %d, chat %d: %w", upd.UpdateID, upd.Message.Chat.ID, err)
	}
	return http.StatusOK, nil
}

func (b *bot) processMessage(ctx context.Context, msg *message) error {
	if !b.isAllowed(msg) {
		b.log.Info("ignored message from a chat that is not allowed", "chat_id", msg.Chat.ID)
		return nil
	}
	if file := msg.mediaFile(); file != nil {
		return b.startTranscription(ctx, msg, file)
	}
	if preview, ok := createPreview(msg.Text); ok {
		_, err := b.telegram.sendMessage(ctx, msg.Chat.ID, preview)
		return err
	}
	return nil
}

// isAllowed decides who may use the bot. TODO: check the users and groups
// allowlist once it exists; until then everyone is allowed.
func (b *bot) isAllowed(*message) bool {
	return true
}

func (m *message) mediaFile() *media {
	switch {
	case m.Voice != nil:
		return m.Voice
	case m.Audio != nil:
		return m.Audio
	case m.VideoNote != nil:
		return m.VideoNote
	}
	return nil
}

// startTranscription hands the audio to AssemblyAI. The result arrives later at
// assemblyPath, so nothing is held in memory while the transcription runs.
func (b *bot) startTranscription(ctx context.Context, msg *message, file *media) (err error) {
	chatID := msg.Chat.ID
	waitID, err := b.telegram.sendMessage(ctx, chatID, waitMessage)
	if err != nil {
		return err
	}

	if file.FileSize > maxTelegramFile {
		return b.telegram.editMessage(ctx, chatID, waitID, tooBigMessage)
	}

	defer func() {
		if err != nil {
			err = errors.Join(err, b.telegram.editMessage(ctx, chatID, waitID, errorMessage))
		}
	}()

	audioURL, err := b.transferAudio(ctx, file.FileID)
	if err != nil {
		return err
	}
	id, err := b.assembly.submit(ctx, audioURL, b.resultWebhookURL(chatID, waitID, messageAuthor(msg)))
	if err != nil {
		return err
	}
	b.log.Info("transcription started", "chat_id", chatID, "transcript_id", id)
	return nil
}

// transferAudio streams the file from Telegram to AssemblyAI without touching
// the disk. Sending AssemblyAI a Telegram link would expose the bot token.
func (b *bot) transferAudio(ctx context.Context, fileID string) (string, error) {
	filePath, err := b.telegram.getFilePath(ctx, fileID)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(ctx, b.cfg.UploadTimeout)
	defer cancel()

	var uploadURL string
	err = b.assembly.retry.do(ctx, func(ctx context.Context) error {
		body, size, err := b.telegram.openFile(ctx, filePath)
		if err != nil {
			return fmt.Errorf("download audio: %w", err)
		}
		defer body.Close()
		uploadURL, err = b.assembly.upload(ctx, body, size)
		return err
	})
	return uploadURL, err
}

func (b *bot) resultWebhookURL(chatID, messageID int64, author string) string {
	query := url.Values{
		queryChatID:    {strconv.FormatInt(chatID, 10)},
		queryMessageID: {strconv.FormatInt(messageID, 10)},
		queryAuthor:    {author},
	}
	return b.cfg.BotURL + assemblyPath + "?" + query.Encode()
}

// handleTranscript delivers a finished transcript. A 5xx makes AssemblyAI
// redeliver the webhook, so it is used only for failures that may pass.
func (b *bot) handleTranscript(r *http.Request) (int, error) {
	if !b.authorized(r.Header.Get(assemblyAuthHeader)) {
		b.log.Warn("rejected assemblyai webhook: bad secret")
		return http.StatusUnauthorized, nil
	}

	query := r.URL.Query()
	chatID, chatErr := strconv.ParseInt(query.Get(queryChatID), 10, 64)
	messageID, messageErr := strconv.ParseInt(query.Get(queryMessageID), 10, 64)
	var payload struct {
		TranscriptID string `json:"transcript_id"`
	}
	bodyErr := json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxWebhookBody)).Decode(&payload)
	if err := errors.Join(chatErr, messageErr, bodyErr); err != nil {
		return http.StatusBadRequest, fmt.Errorf("bad assemblyai webhook: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), assemblyDeadline)
	defer cancel()

	text, err := b.resultText(ctx, payload.TranscriptID, query.Get(queryAuthor))
	if isRetryable(err) {
		return http.StatusInternalServerError, err
	}
	if err != nil {
		text = errorMessage
	}

	editErr := b.telegram.editMessage(ctx, chatID, messageID, text)
	err = errors.Join(err, editErr)
	if isRetryable(editErr) {
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, err
}

func (b *bot) resultText(ctx context.Context, id, author string) (string, error) {
	result, err := b.assembly.transcript(ctx, id)
	if err != nil {
		return "", err
	}
	if result.Status != transcriptCompleted {
		return "", fmt.Errorf("transcript %s has status %q: %s", id, result.Status, result.Error)
	}
	if result.Text == "" {
		return noSpeechMessage, nil
	}
	return truncateMessage(fmt.Sprintf(saidTemplate, author, result.Text)), nil
}

// messageAuthor names whoever originally said the message.
func messageAuthor(m *message) string {
	if origin := m.ForwardOrigin; origin != nil {
		switch origin.Type {
		case "user":
			if name := userName(origin.SenderUser); name != "" {
				return name
			}
		case "hidden_user":
			if origin.SenderUserName != "" {
				return origin.SenderUserName
			}
		case "chat":
			if name := chatName(origin.SenderChat); name != "" {
				return name
			}
		case "channel":
			if name := chatName(origin.Chat); name != "" {
				return name
			}
		}
	}
	if name := userName(m.From); name != "" {
		return name
	}
	if name := chatName(m.SenderChat); name != "" {
		return name
	}
	return unknownAuthor
}

func userName(u *user) string {
	switch {
	case u == nil:
		return ""
	case u.Username != "":
		return "@" + u.Username
	}
	return u.FirstName
}

func chatName(c *chat) string {
	switch {
	case c == nil:
		return ""
	case c.Username != "":
		return "@" + c.Username
	}
	return c.Title
}

// truncateMessage fits text into Telegram's message limit. Strings travel as
// UTF-8, but the limit is in characters, not bytes. Telegram counts entities in
// UTF-16 units; that count is never below the number of characters, so it is
// safe under either reading of the limit, and cutting between runes keeps the
// text valid.
func truncateMessage(text string) string {
	limit := maxMessageLength - len(ellipsis)
	units, cut := 0, -1
	for i, r := range text {
		if cut < 0 && units+runeUnits(r) > limit {
			cut = i
		}
		units += runeUnits(r)
		if units > maxMessageLength {
			return text[:cut] + ellipsis
		}
	}
	return text
}

func runeUnits(r rune) int {
	if n := utf16.RuneLen(r); n > 0 {
		return n
	}
	return 1
}
