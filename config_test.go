package main

import (
	"strings"
	"testing"
	"time"
)

func env(overrides map[string]string) func(string) string {
	values := map[string]string{
		"BOT_TOKEN":      "token",
		"BOT_URL":        "https://bot.example.com/",
		"ASSEMBLY_KEY":   "key",
		"WEBHOOK_SECRET": "secret_1-2",
	}
	for k, v := range overrides {
		values[k] = v
	}
	return func(key string) string { return values[key] }
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := loadConfig(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BotURL != "https://bot.example.com" {
		t.Errorf("BotURL = %q, trailing slash must be removed", cfg.BotURL)
	}
	if cfg.Port != "8080" || cfg.RetryAttempts != 3 || cfg.RetryDelay != 5*time.Second ||
		cfg.UploadTimeout != time.Minute || cfg.TelegramTimeout != 10*time.Second {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
	if len(cfg.SpeechModels) != 2 {
		t.Errorf("SpeechModels = %v", cfg.SpeechModels)
	}
}

func TestLoadConfigReportsEveryProblem(t *testing.T) {
	_, err := loadConfig(func(string) string { return "" })
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, key := range []string{"BOT_TOKEN", "BOT_URL", "ASSEMBLY_KEY", "WEBHOOK_SECRET"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error %q does not mention %s", err, key)
		}
	}
}

func TestLoadConfigRejectsBadValues(t *testing.T) {
	tests := map[string]map[string]string{
		"duration":  {"TELEGRAM_TIMEOUT": "10"},
		"negative":  {"RETRY_DELAY": "-1s"},
		"attempts":  {"RETRY_ATTEMPTS": "0"},
		"port":      {"PORT": "http"},
		"log level": {"LOG_LEVEL": "loud"},
		"http url":  {"BOT_URL": "http://bot.example.com"},
		"secret":    {"WEBHOOK_SECRET": "bad secret!"},
		"no models": {"ASSEMBLY_SPEECH_MODELS": " , "},
	}
	for name, overrides := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := loadConfig(env(overrides)); err == nil {
				t.Errorf("expected an error for %v", overrides)
			}
		})
	}
}

func TestLoadConfigDoesNotPrintSecret(t *testing.T) {
	_, err := loadConfig(env(map[string]string{"WEBHOOK_SECRET": "bad secret!"}))
	if err == nil || strings.Contains(err.Error(), "bad secret!") {
		t.Errorf("error must not contain the secret: %v", err)
	}
}
