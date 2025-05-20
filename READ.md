# Sankarea

Minimal political-news Discord bot in Go.

## Setup

1. **Configure** `config/sources.yml` with your RSS & HTML sources.
2. **Add** these GitHub secrets:
   - `OPENAI_API_KEY`
   - `DISCORD_WEBHOOKS` (comma-separated webhook URLs)
3. **Push** to GitHub.

## Manual Run

```bash
go run cmd/sankarea/main.go --config config/sources.yml --state data/state.json
