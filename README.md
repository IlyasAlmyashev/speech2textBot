Speech2text telegram bot for voice messages, audio files and video notes.
It also rewrites Instagram and X/Twitter links into versions with working previews.

## How it works

The bot is stateless and does all its work inside incoming requests, so it can
run on Cloud Run with request-based billing and scale to zero.

1. Telegram posts an update to `/callback/update`.
2. The bot sends a "please wait" message, streams the file from Telegram to
   AssemblyAI and submits a transcription with a webhook URL.
3. AssemblyAI posts to `/callback/assembly` when it finishes.
4. The bot fetches the transcript and edits the "please wait" message.

Both webhooks are authenticated with `WEBHOOK_SECRET`.

## Configuration

Required variables (the bot panics on startup without them):

| Variable | Description |
|---|---|
| `BOT_TOKEN` | get it from https://t.me/BotFather |
| `BOT_URL` | public https URL, from the deployment provider or ngrok if you run locally |
| `ASSEMBLY_KEY` | get it from https://www.assemblyai.com |
| `WEBHOOK_SECRET` | 1-256 characters of `A-Za-z0-9_-`, for example `openssl rand -hex 32` |

Optional variables:

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | set by Cloud Run |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error` |
| `TELEGRAM_TIMEOUT` | `10s` | one Telegram API attempt |
| `ASSEMBLY_TIMEOUT` | `30s` | one AssemblyAI API attempt |
| `UPLOAD_TIMEOUT` | `60s` | whole audio transfer, retries included |
| `RETRY_ATTEMPTS` | `3` | attempts for transient failures |
| `RETRY_DELAY` | `5s` | pause between attempts |
| `SHUTDOWN_TIMEOUT` | `8s` | Cloud Run allows 10s after SIGTERM |
| `ASSEMBLY_SPEECH_MODELS` | `universal-3-5-pro,universal-2` | models in priority order |

## Run

```
go test ./...

docker build -t s2tbot .
docker run -d -e BOT_TOKEN='xxx' -e BOT_URL='xxx' -e ASSEMBLY_KEY='xxx' -e WEBHOOK_SECRET='xxx' -p 8080:8080 s2tbot
```

forked from https://github.com/psorokin02/telegram_chatGPT_bot
