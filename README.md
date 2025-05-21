# Sankarea

**Automated Discord news & fact-checking bot.**

## Required Environment Variables

Set these in your VPS, Docker, or GitHub Actions secrets:

- CLAIMBUSTER_API_KEY
- DISCORD_APPLICATION_ID
- DISCORD_BOT_TOKEN
- DISCORD_CHANNEL_ID
- DISCORD_GUILD_ID
- DISCORD_WEBHOOKS
- GOOGLE_FACTCHECK_API_KEY
- OPENAI_API_KEY

## Quick Start

1. Fill out `config/sources.yml` (add/remove news sources).
2. `data/state.json` can be empty as above.
3. Set all required secrets/env vars.
4. Build and run:

    ```
    cd cmd/sankarea
    go run main.go
    ```

    or use Docker/Makefile.

## Expansion

This repo will support:
- Automated news posting grouped by bias
- Dynamic Discord slash commands for adding/removing sources
- Built-in fact-check via Google/ClaimBuster APIs

**No config files contain secrets. All secrets come from environment variables.**
