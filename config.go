package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// webhookSecretPattern is the character set Telegram accepts for secret_token.
var webhookSecretPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,256}$`)

// Config holds everything read from the environment.
type Config struct {
	BotToken      string
	BotURL        string
	AssemblyKey   string
	WebhookSecret string

	Port            string
	LogLevel        slog.Level
	TelegramTimeout time.Duration // one Telegram API attempt
	AssemblyTimeout time.Duration // one AssemblyAI API attempt
	UploadTimeout   time.Duration // whole audio transfer, retries included
	RetryAttempts   int
	RetryDelay      time.Duration
	ShutdownTimeout time.Duration
	SpeechModels    []string
}

// loadConfig reads and validates the environment. It reports every problem at
// once, so a misconfigured deployment can be fixed in a single pass.
func loadConfig(getenv func(string) string) (Config, error) {
	var errs []error

	lookup := func(key, fallback string) string {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
		return fallback
	}
	required := func(key string) string {
		v := lookup(key, "")
		if v == "" {
			errs = append(errs, fmt.Errorf("%s is required", key))
		}
		return v
	}
	duration := func(key, fallback string) time.Duration {
		raw := lookup(key, fallback)
		d, err := time.ParseDuration(raw)
		if err != nil || d <= 0 {
			errs = append(errs, fmt.Errorf("%s must be a positive duration, got %q", key, raw))
		}
		return d
	}

	cfg := Config{
		BotToken:        required("BOT_TOKEN"),
		BotURL:          strings.TrimRight(required("BOT_URL"), "/"),
		AssemblyKey:     required("ASSEMBLY_KEY"),
		WebhookSecret:   required("WEBHOOK_SECRET"),
		Port:            lookup("PORT", "8080"),
		TelegramTimeout: duration("TELEGRAM_TIMEOUT", "10s"),
		AssemblyTimeout: duration("ASSEMBLY_TIMEOUT", "30s"),
		UploadTimeout:   duration("UPLOAD_TIMEOUT", "60s"),
		RetryDelay:      duration("RETRY_DELAY", "5s"),
		ShutdownTimeout: duration("SHUTDOWN_TIMEOUT", "8s"),
	}

	if cfg.BotURL != "" {
		if u, err := url.Parse(cfg.BotURL); err != nil || u.Scheme != "https" || u.Host == "" {
			errs = append(errs, errors.New("BOT_URL must be an https URL"))
		}
	}
	if cfg.WebhookSecret != "" && !webhookSecretPattern.MatchString(cfg.WebhookSecret) {
		errs = append(errs, errors.New("WEBHOOK_SECRET must be 1-256 characters of A-Za-z0-9_-"))
	}
	if port, err := strconv.Atoi(cfg.Port); err != nil || port < 1 || port > 65535 {
		errs = append(errs, fmt.Errorf("PORT must be a port number, got %q", cfg.Port))
	}

	rawAttempts := lookup("RETRY_ATTEMPTS", "3")
	attempts, err := strconv.Atoi(rawAttempts)
	if err != nil || attempts < 1 {
		errs = append(errs, fmt.Errorf("RETRY_ATTEMPTS must be a positive integer, got %q", rawAttempts))
	}
	cfg.RetryAttempts = attempts

	rawLevel := lookup("LOG_LEVEL", "info")
	if err := cfg.LogLevel.UnmarshalText([]byte(rawLevel)); err != nil {
		errs = append(errs, fmt.Errorf("LOG_LEVEL must be debug, info, warn or error, got %q", rawLevel))
	}

	for _, model := range strings.Split(lookup("ASSEMBLY_SPEECH_MODELS", "universal-3-5-pro,universal-2"), ",") {
		if model = strings.TrimSpace(model); model != "" {
			cfg.SpeechModels = append(cfg.SpeechModels, model)
		}
	}
	if len(cfg.SpeechModels) == 0 {
		errs = append(errs, errors.New("ASSEMBLY_SPEECH_MODELS must list at least one model"))
	}

	return cfg, errors.Join(errs...)
}
