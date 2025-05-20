# GoNewsBot

A zero-cost, super-fast political news Discord bot written entirely in Go.

## Quickstart

```bash
# Local dry-run
go run ./cmd/newsbot --config config/sources.yml --state data/state.json --dry-run

# Build & run
go build -o newsbot ./cmd/newsbot
./newsbot --config config/sources.yml --state data/state.json
```

## Features

- Concurrent RSS & HTML scraping
- ETag-based caching with state.json and GitHub Actions cache
- Deduplication & TTL via JSON state
- Title clustering and trending detection
- Batch summarization via OpenAI GPT-3.5-turbo
- Configurable keyword watchlist
- Rich Discord embeds
- Fact-checking via Google Fact Check + ClaimBuster
- Prometheus monitoring endpoint
