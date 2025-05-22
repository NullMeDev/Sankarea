# Sankarea

**Automated Discord news & fact-checking bot.**

Sankarea is a Discord bot that aggregates news from RSS feeds, performs fact-checking, and provides daily news digests to Discord channels. The bot is built in Go and features content moderation, scheduled updates, and administration through Discord slash commands.

## Required Environment Variables

Set these in your VPS, Docker, or GitHub Actions secrets:

- `DISCORD_BOT_TOKEN` - Your Discord bot token
- `DISCORD_APPLICATION_ID` - Your Discord application ID
- `DISCORD_GUILD_ID` - Your Discord server (guild) ID
- `DISCORD_CHANNEL_ID` - Channel where news will be posted
- `DISCORD_WEBHOOKS` - Comma-separated webhook URLs for notifications
- `OPENAI_API_KEY` - For content moderation and summarization
- `GOOGLE_FACTCHECK_API_KEY` - For fact checking via Google Fact Check API
- `CLAIMBUSTER_API_KEY` - For claim detection via ClaimBuster API

## Quick Start

1. Fill out `config/sources.yml` with your news sources:
   ```yaml
   - name: "CNN"
     url: "http://rss.cnn.com/rss/cnn_topstories.rss"
     category: "Mainstream"
     bias: "Left-Center"
     factCheckAuto: true

   - name: "Fox News"
     url: "http://feeds.foxnews.com/foxnews/latest"
     category: "Mainstream"
     bias: "Right"
     factCheckAuto: true
   ```

2. Set all required environment variables.

3. Build and run:

   ```bash
   cd cmd/sankarea
   go run main.go
   ```

   or use Docker:

   ```bash
   docker build -t sankarea .
   docker run -v $(pwd)/config:/app/config -v $(pwd)/data:/app/data sankarea
   ```

## Features

- **News Aggregation**: Collects news from multiple RSS feeds
- **Categorized Reporting**: Groups news by category and bias
- **Fact Checking**: Automatically checks claims using Google Fact Check & ClaimBuster APIs
- **Content Moderation**: Filters inappropriate content
- **Daily Digest**: Creates summarized daily news briefings
- **Discord Commands**: Provides slash commands for controlling the bot

## Configuration

All configuration is stored in:
- `config/config.json` - Bot configuration
- `config/sources.yml` - News sources
- `data/state.json` - Runtime state

**No config files contain secrets. All secrets come from environment variables.**

## Administration

The bot provides several slash commands:
- `/ping` - Check if the bot is alive
- `/status` - Show current bot status
- `/version` - Show bot version
- `/source add` - Add a news source
- `/source remove` - Remove a news source
- `/source list` - List all sources

## Development

To contribute to Sankarea:

1. Fork the repository
2. Make your changes
3. Submit a pull request

## License

MIT License
