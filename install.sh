#!/bin/bash
# Sankarea Installation Script
# Created by NullMeDev on 2025-05-22

echo "🔧 Setting up Sankarea Discord News Bot..."

# Check for Go installation
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.21 or later."
    exit 1
fi

# Check Go version
GO_VERSION=$(go version | grep -oP 'go\K[0-9]+\.[0-9]+')
if (( $(echo "$GO_VERSION < 1.21" | bc -l) )); then
    echo "❌ Go version $GO_VERSION is too old. Please upgrade to Go 1.21 or later."
    exit 1
fi

# Create directories if they don't exist
mkdir -p config data logs

# Check if config exists, create if not
if [ ! -f "config/config.json" ]; then
    echo "Creating default configuration file..."
    cat > config/config.json << EOF
{
    "version": "1.0.0",
    "maxPostsPerSource": 5,
    "newsIntervalMinutes": 120,
    "digestCronSchedule": "0 8 * * *",
    "news15MinCron": "*/15 * * * *",
    "userAgentString": "Sankarea News Bot v1.0.0",
    "enableFactCheck": true,
    "enableSummarization": true,
    "enableContentFiltering": true,
    "reports": {
        "enabled": true,
        "weeklyCron": "0 9 * * 1",
        "monthlyCron": "0 9 1 * *"
    }
}
EOF
fi

# Check for env file, create if not
if [ ! -f ".env" ]; then
    echo "Creating example .env file..."
    cat > .env << EOF
# Discord Configuration
DISCORD_BOT_TOKEN=your_discord_bot_token_here
DISCORD_APPLICATION_ID=your_discord_application_id_here
DISCORD_GUILD_ID=your_discord_guild_id_here
DISCORD_CHANNEL_ID=your_discord_channel_id_here
DISCORD_WEBHOOKS=comma,separated,webhook,urls

# API Keys
OPENAI_API_KEY=your_openai_api_key_here
GOOGLE_FACTCHECK_API_KEY=your_google_factcheck_api_key_here
CLAIMBUSTER_API_KEY=your_claimbuster_api_key_here
YOUTUBE_API_KEY=your_youtube_api_key_here
TWITTER_BEARER_TOKEN=your_twitter_bearer_token_here
EOF
    echo "⚠️  Please edit the .env file with your actual API keys and tokens"
fi

# Install dependencies
echo "Installing dependencies..."
go mod init sankarea
go mod tidy
go get github.com/bwmarrin/discordgo
go get github.com/mmcdole/gofeed
go get github.com/robfig/cron/v3
go get github.com/sashabaranov/go-openai
go get github.com/mattn/go-sqlite3
go get gopkg.in/yaml.v2

echo "Building application..."
go build -o bin/sankarea cmd/sankarea/*.go

echo "✅ Setup complete!"
echo "Run the bot with ./bin/sankarea"
