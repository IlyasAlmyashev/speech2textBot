package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

const (
	testToken  = "123:SECRET-TOKEN"
	testSecret = "webhook-secret"
	testAudio  = "fake ogg bytes"
)

// fakeAPIs stands in for Telegram and AssemblyAI at once.
type fakeAPIs struct {
	t *testing.T

	mu       sync.Mutex
	calls    []string // Telegram methods in call order
	sent     []string // sendMessage texts
	edits    []string // editMessageText texts
	uploaded string
	submit   map[string]any

	transcript   transcript
	getFileFail  bool
	editStatus   int
	editMessage  string
	getStatus    int
	submitStatus int
}

func (f *fakeAPIs) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	switch {
	case r.URL.Path == "/file/bot"+testToken+"/voice/file.oga":
		io.WriteString(w, testAudio)
	case strings.HasPrefix(r.URL.Path, "/bot"+testToken+"/"):
		f.telegram(w, r, strings.TrimPrefix(r.URL.Path, "/bot"+testToken+"/"))
	case r.URL.Path == "/v2/upload":
		raw, _ := io.ReadAll(r.Body)
		f.uploaded = string(raw)
		json.NewEncoder(w).Encode(map[string]string{"upload_url": "https://cdn.assemblyai.test/1"})
	case r.URL.Path == "/v2/transcript":
		if f.submitStatus != 0 {
			w.WriteHeader(f.submitStatus)
			io.WriteString(w, `{"error":"rejected"}`)
			return
		}
		json.NewDecoder(r.Body).Decode(&f.submit)
		json.NewEncoder(w).Encode(transcript{ID: "tr1", Status: "queued"})
	case strings.HasPrefix(r.URL.Path, "/v2/transcript/"):
		if f.getStatus != 0 {
			w.WriteHeader(f.getStatus)
			return
		}
		json.NewEncoder(w).Encode(f.transcript)
	default:
		f.t.Errorf("unexpected request: %s", r.URL.Path)
	}
}

func (f *fakeAPIs) telegram(w http.ResponseWriter, r *http.Request, method string) {
	var payload map[string]any
	json.NewDecoder(r.Body).Decode(&payload)
	f.calls = append(f.calls, method)
	text, _ := payload["text"].(string)

	switch method {
	case "sendMessage":
		f.sent = append(f.sent, text)
		io.WriteString(w, `{"ok":true,"result":{"message_id":77}}`)
	case "editMessageText":
		if f.editStatus != 0 {
			w.WriteHeader(f.editStatus)
			json.NewEncoder(w).Encode(map[string]any{"ok": false, "description": f.editMessage})
			return
		}
		f.edits = append(f.edits, text)
		io.WriteString(w, `{"ok":true,"result":{}}`)
	case "getFile":
		if f.getFileFail {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, `{"ok":false,"description":"Bad Request: file is too big"}`)
			return
		}
		io.WriteString(w, `{"ok":true,"result":{"file_path":"voice/file.oga"}}`)
	default:
		io.WriteString(w, `{"ok":true,"result":true}`)
	}
}

func newTestBot(t *testing.T, logs io.Writer) (*bot, *fakeAPIs, http.Handler) {
	t.Helper()
	fake := &fakeAPIs{t: t}
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)

	cfg := Config{
		BotToken:        testToken,
		BotURL:          "https://bot.example.com",
		AssemblyKey:     "aai-key",
		WebhookSecret:   testSecret,
		TelegramTimeout: time.Second,
		AssemblyTimeout: time.Second,
		UploadTimeout:   time.Second,
		RetryAttempts:   2,
		RetryDelay:      time.Millisecond,
		SpeechModels:    []string{"universal-2"},
	}
	if logs == nil {
		logs = io.Discard
	}
	b := newBot(cfg, slog.New(slog.NewJSONHandler(logs, nil)))
	b.telegram.baseURL = server.URL
	b.assembly.baseURL = server.URL
	return b, fake, b.routes()
}

func post(h http.Handler, target, header, secret, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	req.Header.Set(header, secret)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func postUpdate(h http.Handler, body string) *httptest.ResponseRecorder {
	return post(h, updatePath, secretHeader, testSecret, body)
}

func TestWebhooksRequireTheSecret(t *testing.T) {
	_, fake, h := newTestBot(t, nil)

	for _, secret := range []string{"", "wrong"} {
		if rec := post(h, updatePath, secretHeader, secret, `{"message":{"text":"https://x.com/a"}}`); rec.Code != http.StatusUnauthorized {
			t.Errorf("update with secret %q: status %d", secret, rec.Code)
		}
		target := assemblyPath + "?chat_id=1&message_id=2"
		if rec := post(h, target, assemblyAuthHeader, secret, `{"transcript_id":"tr1"}`); rec.Code != http.StatusUnauthorized {
			t.Errorf("assembly webhook with secret %q: status %d", secret, rec.Code)
		}
	}
	if len(fake.calls) != 0 {
		t.Errorf("rejected requests reached Telegram: %v", fake.calls)
	}
}

func TestLinkPreviewIsSentDirectly(t *testing.T) {
	_, fake, h := newTestBot(t, nil)

	rec := postUpdate(h, `{"update_id":1,"message":{"chat":{"id":5},"text":"https://x.com/a"}}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if len(fake.sent) != 1 || fake.sent[0] != "here's your preview : https://www.fixupx.com/a" {
		t.Errorf("sent = %v", fake.sent)
	}
	if len(fake.edits) != 0 {
		t.Errorf("preview must not use the wait message, edits = %v", fake.edits)
	}
}

func TestMediaStartsTranscription(t *testing.T) {
	for _, field := range []string{"voice", "audio", "video_note"} {
		t.Run(field, func(t *testing.T) {
			_, fake, h := newTestBot(t, nil)

			rec := postUpdate(h, `{"message":{"chat":{"id":5},"from":{"username":"bob","first_name":"Bob"},`+
				`"`+field+`":{"file_id":"f1","file_size":10}}}`)

			if rec.Code != http.StatusOK {
				t.Fatalf("status %d", rec.Code)
			}
			if len(fake.sent) != 1 || fake.sent[0] != waitMessage {
				t.Errorf("sent = %v", fake.sent)
			}
			if fake.uploaded != testAudio {
				t.Errorf("uploaded %q", fake.uploaded)
			}
			if got := fake.submit["audio_url"]; got != "https://cdn.assemblyai.test/1" {
				t.Errorf("audio_url = %v, must be the upload URL", got)
			}
			if got := fake.submit["webhook_auth_header_value"]; got != testSecret {
				t.Errorf("webhook auth value = %v", got)
			}

			hook, err := url.Parse(fake.submit["webhook_url"].(string))
			if err != nil {
				t.Fatal(err)
			}
			query := hook.Query()
			if hook.Path != assemblyPath || query.Get(queryChatID) != "5" ||
				query.Get(queryMessageID) != "77" || query.Get(queryAuthor) != "@bob" {
				t.Errorf("webhook_url = %s", hook)
			}
		})
	}
}

func TestOversizedFileIsRefusedWithoutDownloading(t *testing.T) {
	_, fake, h := newTestBot(t, nil)

	postUpdate(h, `{"message":{"chat":{"id":5},"audio":{"file_id":"f1","file_size":20971521}}}`)

	if len(fake.edits) != 1 || fake.edits[0] != tooBigMessage {
		t.Errorf("edits = %v", fake.edits)
	}
	if fake.uploaded != "" || fake.submit != nil {
		t.Error("oversized file must not be transferred")
	}
}

func TestFailedStartReportsErrorToUserAndAcknowledges(t *testing.T) {
	tests := map[string]func(*fakeAPIs){
		"getFile fails": func(f *fakeAPIs) { f.getFileFail = true },
		"submit fails":  func(f *fakeAPIs) { f.submitStatus = http.StatusBadRequest },
	}
	for name, setup := range tests {
		t.Run(name, func(t *testing.T) {
			_, fake, h := newTestBot(t, nil)
			setup(fake)

			rec := postUpdate(h, `{"message":{"chat":{"id":5},"voice":{"file_id":"f1"}}}`)

			if rec.Code != http.StatusOK {
				t.Errorf("status %d: Telegram would redeliver the update", rec.Code)
			}
			if len(fake.edits) != 1 || fake.edits[0] != errorMessage {
				t.Errorf("edits = %v", fake.edits)
			}
		})
	}
}

func deliver(h http.Handler) *httptest.ResponseRecorder {
	target := assemblyPath + "?chat_id=5&message_id=77&author=" + url.QueryEscape("@bob")
	return post(h, target, assemblyAuthHeader, testSecret, `{"transcript_id":"tr1","status":"completed"}`)
}

func TestTranscriptDelivery(t *testing.T) {
	tests := []struct {
		name       string
		transcript transcript
		want       string
	}{
		{
			name:       "completed",
			transcript: transcript{Status: "completed", Text: "привет"},
			want:       "@bob said: \n\nпривет",
		},
		{
			name:       "no speech",
			transcript: transcript{Status: "completed"},
			want:       noSpeechMessage,
		},
		{
			name:       "transcription error",
			transcript: transcript{Status: "error", Error: "boom"},
			want:       errorMessage,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, fake, h := newTestBot(t, nil)
			fake.transcript = tt.transcript

			rec := deliver(h)

			if rec.Code != http.StatusOK {
				t.Fatalf("status %d", rec.Code)
			}
			if len(fake.edits) != 1 || fake.edits[0] != tt.want {
				t.Errorf("edits = %q, want %q", fake.edits, tt.want)
			}
		})
	}
}

func TestTranscriptDeliveryStatus(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*fakeAPIs)
		want  int
	}{
		{
			name:  "temporary AssemblyAI failure is redelivered",
			setup: func(f *fakeAPIs) { f.getStatus = http.StatusBadGateway },
			want:  http.StatusInternalServerError,
		},
		{
			name:  "temporary Telegram failure is redelivered",
			setup: func(f *fakeAPIs) { f.editStatus = http.StatusBadGateway },
			want:  http.StatusInternalServerError,
		},
		{
			name: "repeated edit counts as success",
			setup: func(f *fakeAPIs) {
				f.editStatus, f.editMessage = http.StatusBadRequest, "Bad Request: message is not modified"
			},
			want: http.StatusOK,
		},
		{
			name: "deleted message is not redelivered",
			setup: func(f *fakeAPIs) {
				f.editStatus, f.editMessage = http.StatusBadRequest, "Bad Request: message to edit not found"
			},
			want: http.StatusOK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, fake, h := newTestBot(t, nil)
			fake.transcript = transcript{Status: "completed", Text: "hi"}
			tt.setup(fake)

			if rec := deliver(h); rec.Code != tt.want {
				t.Errorf("status %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

func TestMalformedAssemblyWebhookIsPermanentFailure(t *testing.T) {
	_, _, h := newTestBot(t, nil)
	rec := post(h, assemblyPath+"?chat_id=x", assemblyAuthHeader, testSecret, `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status %d", rec.Code)
	}
}

func TestPanicIsLoggedAndAnswered(t *testing.T) {
	var logs bytes.Buffer
	b, _, _ := newTestBot(t, &logs)

	h := b.endpoint("test", func(*http.Request) (int, error) { panic("boom") })
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status %d", rec.Code)
	}
	if !strings.Contains(logs.String(), "boom") {
		t.Errorf("panic not logged: %s", logs.String())
	}
}

func TestBotTokenNeverReachesLogs(t *testing.T) {
	var logs bytes.Buffer
	b, _, h := newTestBot(t, &logs)
	b.telegram.baseURL = "http://127.0.0.1:1" // nothing listens here

	postUpdate(h, `{"message":{"chat":{"id":5},"text":"https://x.com/a"}}`)

	if !strings.Contains(logs.String(), "request failed") {
		t.Fatalf("failure not logged: %s", logs.String())
	}
	if strings.Contains(logs.String(), testToken) {
		t.Errorf("token leaked: %s", logs.String())
	}
}

func TestRetrierStopsOnPermanentErrors(t *testing.T) {
	permanent := &apiError{Status: http.StatusBadRequest}
	calls := 0
	err := retrier{attempts: 3}.do(context.Background(), func(context.Context) error {
		calls++
		return permanent
	})
	if !errors.Is(err, permanent) || calls != 1 {
		t.Errorf("err = %v after %d calls", err, calls)
	}
}

func TestRetrierRepeatsTransientErrors(t *testing.T) {
	calls := 0
	err := retrier{attempts: 3}.do(context.Background(), func(context.Context) error {
		calls++
		if calls < 3 {
			return &apiError{Status: http.StatusTooManyRequests}
		}
		return nil
	})
	if err != nil || calls != 3 {
		t.Errorf("err = %v after %d calls", err, calls)
	}
}

func TestMessageAuthor(t *testing.T) {
	bob := &user{Username: "bob", FirstName: "Bob"}
	alice := &user{FirstName: "Alice"}
	tests := []struct {
		name string
		msg  message
		want string
	}{
		{"username", message{From: bob}, "@bob"},
		{"first name", message{From: alice}, "Alice"},
		{"forwarded user", message{From: bob, ForwardOrigin: &messageOrigin{Type: "user", SenderUser: alice}}, "Alice"},
		{"forwarded hidden user", message{From: bob, ForwardOrigin: &messageOrigin{Type: "hidden_user", SenderUserName: "Carol"}}, "Carol"},
		{"forwarded chat", message{From: bob, ForwardOrigin: &messageOrigin{Type: "chat", SenderChat: &chat{Title: "Team"}}}, "Team"},
		{"forwarded channel", message{From: bob, ForwardOrigin: &messageOrigin{Type: "channel", Chat: &chat{Username: "news"}}}, "@news"},
		{"unknown origin", message{From: bob, ForwardOrigin: &messageOrigin{Type: "future"}}, "@bob"},
		{"anonymous admin", message{SenderChat: &chat{Title: "Group"}}, "Group"},
		{"nobody", message{}, unknownAuthor},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := messageAuthor(&tt.msg); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTruncateMessage(t *testing.T) {
	tests := []struct {
		name string
		text string
		cut  bool
	}{
		{"short", "привет", false},
		{"exactly the limit in cyrillic", strings.Repeat("я", maxMessageLength), false},
		{"over the limit in cyrillic", strings.Repeat("я", maxMessageLength+1), true},
		{"emoji count double", strings.Repeat("😀", maxMessageLength/2+1), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateMessage(tt.text)
			if units := len(utf16.Encode([]rune(got))); units > maxMessageLength {
				t.Errorf("result has %d UTF-16 units", units)
			}
			if !utf8.ValidString(got) {
				t.Error("result is not valid UTF-8")
			}
			if cut := got != tt.text; cut != tt.cut {
				t.Errorf("cut = %v, want %v", cut, tt.cut)
			}
			if tt.cut && !strings.HasSuffix(got, ellipsis) {
				t.Error("cut text must end with an ellipsis")
			}
		})
	}
}
