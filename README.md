# Sankarea Discord News Bot

![Version](https://img.shields.io/badge/version-1.0.0-blue)
![Go](https://img.shields.io/badge/go-1.21+-00ADD8?logo=go)
![License](https://img.shields.io/github/license/NullMeDev/sankarea)

Sankarea is a versatile Discord news bot that aggregates content from RSS feeds, performs fact-checking, generates news digests, and offers moderation capabilities. It provides balanced news coverage from diverse sources across the political spectrum.

**Last updated:** 2025-05-22 15:26:18 (UTC) by NullMeDev

## 📋 Features

- **News Aggregation**: Collects and posts news from 37+ pre-configured sources
- **Balanced Coverage**: Includes sources across the political spectrum (Left, Center, Right)
- **Auto Fact-Checking**: Verifies news claims using external fact-checking services
- **Content Analysis**: Detects and tags potentially misleading content
- **News Digests**: Generates daily summaries of top stories
- **Trending Topics**: Identifies and highlights emerging news trends
- **Multiple Categories**: Covers Politics, Business, Technology, Health, Science, and more
- **User Management**: Moderate your Discord server with kick, ban, mute commands
- **Customizable Themes**: Switch between visual themes (light/dark) for embeds
- **Detailed Reports**: Weekly and monthly reports on news coverage
- **Performance Monitoring**: Web dashboard to track system health, usage, errors
- **Documentation Generator**: Build internal docs via `go run tools/generate_docs.go`

## 🚀 Getting Started

### Prerequisites

- Go 1.21 or later
- Discord Bot Token
- Discord Server with admin permissions

### Installation

#### Quick Install
```bash
# Clone the repository
git clone https://github.com/NullMeDev/sankarea.git
cd sankarea

# Run the installation script
chmod +x install.sh
./install.sh

# Edit the .env file with your Discord bot token and other API keys
nano .env

# Build and run the bot
make run
```

#### Docker Install
```bash
# Clone the repository
git clone https://github.com/NullMeDev/sankarea.git
cd sankarea

# Edit the .env file with your Discord bot token and other API keys
nano .env

# Build and run with Docker Compose
docker-compose up -d
```

## 🤖 Discord Commands

### Basic Commands
| Command   | Description                | Example     |
|-----------|----------------------------|-------------|
| `/ping`   | Check if the bot is alive | `/ping`     |
| `/status` | Show current status        | `/status`   |
| `/version`| Show bot version info      | `/version`  |

### News Source Management
| Command         | Description                         | Example                                               |
|------------------|-------------------------------------|-------------------------------------------------------|
| `/source add`    | Add a new news source               | `/source add name:CNN url:http://rss.cnn.com/rss/cnn_topstories.rss category:Mainstream fact_check:true` |
| `/source remove` | Remove an existing news source      | `/source remove name:CNN`                             |
| `/source list`   | List all news sources               | `/source list`                                        |
| `/source update` | Update an existing news source      | `/source update name:CNN url:http://new.url.com/feed category:News paused:true` |

### Admin Commands
| Command           | Description                          | Example                    |
|------------------|--------------------------------------|----------------------------|
| `/admin pause`   | Pause news gathering                | `/admin pause`            |
| `/admin resume`  | Resume news gathering               | `/admin resume`           |
| `/admin reload`  | Reload configuration                | `/admin reload`           |
| `/admin digest`  | Generate/send digest now            | `/admin digest`           |
| `/admin config`  | View/update config (owner only)     | `/admin config maxPosts:50` |

### Moderation Commands
| Command  | Description              | Example                                                                 |
|----------|--------------------------|-------------------------------------------------------------------------|
| `/kick`  | Kick a user              | `/kick user:@user reason:"Violation" notify:true`                     |
| `/ban`   | Ban a user               | `/ban user:@user reason:"Repeat violations" days:1 notify:true`       |
| `/mute`  | Timeout (mute) a user    | `/mute user:@user reason:"Spam" duration:60 notify:true`              |

## 🔹 Directory Structure
```
sankarea/
├── cmd/
│   └── sankarea/        # Main bot code
├── config/              # Configuration files
│   ├── config.json      # Main configuration
│   ├── sources.yml      # News sources
│   └── themes/          # Visual themes
├── data/                # Data storage
├── logs/                # Log files
├── tools/               # Utility scripts
│   ├── dashboard.html   # Web dashboard
│   ├── validate_feeds.go# RSS validation tool
│   ├── integration_test.go # Integration tests
│   └── generate_docs.go # Documentation generator
├── Dockerfile           # Docker configuration
├── docker-compose.yml   # Docker Compose file
├── Makefile             # Development commands
├── install.sh           # Installation script
└── README.md            # This file
```

## 📄 Configuration

### News Sources (`config/sources.yml`)
The bot comes preloaded with 37+ sources. You can:
- Add new sources: `/source add`
- Remove sources: `/source remove`
- Update sources: `/source update`
- List all sources: `/source list`

### Environment Variables (`.env`)
```env
# Discord Configuration
DISCORD_BOT_TOKEN=your_discord_bot_token
DISCORD_APPLICATION_ID=your_discord_app_id
DISCORD_GUILD_ID=your_discord_guild_id
DISCORD_CHANNEL_ID=your_discord_channel_id
DISCORD_WEBHOOKS=comma,separated,webhooks

# API Keys
OPENAI_API_KEY=your_openai_api_key
GOOGLE_FACTCHECK_API_KEY=your_google_factcheck_api_key
CLAIMBUSTER_API_KEY=your_claimbuster_api_key
YOUTUBE_API_KEY=your_youtube_api_key
TWITTER_BEARER_TOKEN=your_twitter_bearer_token
```

## 🌍 Web Dashboard

The bot includes a live dashboard to monitor:
- System status
- News statistics
- Source health
- API usage
- Error logs
- Discord interactions

Access is provided via `tools/dashboard.html`.

---

For contributions, feature requests, or issues, visit the [GitHub repo](https://github.com/NullMeDev/sankarea).

> Built with ❤️ by NullMeDev
