package main

import (
    "fmt"
    "log"
    "os"
    "path/filepath"
)

func getEnvOrFail(key string) string {
    v := os.Getenv(key)
    if v == "" {
        log.Fatalf("Missing required environment variable: %s", key)
    }
    return v
}

func main() {
    fmt.Println("Sankarea bot starting up... (env var mode)")

    // Check config/sources/data files exist, fail fast if not
    mustExist := []string{
        filepath.Join("..", "..", "config", "config.json"),
        filepath.Join("..", "..", "config", "sources.yml"),
        filepath.Join("..", "..", "data", "state.json"),
    }
    for _, path := range mustExist {
        if _, err := os.Stat(path); os.IsNotExist(err) {
            log.Fatalf("ERROR: Required file not found: %s", path)
        }
    }

    // Pull all critical values from environment variables
    claimBusterAPIKey := getEnvOrFail("CLAIMBUSTER_API_KEY")
    discordAppID := getEnvOrFail("DISCORD_APPLICATION_ID")
    discordBotToken := getEnvOrFail("DISCORD_BOT_TOKEN")
    discordChannelID := getEnvOrFail("DISCORD_CHANNEL_ID")
    discordGuildID := getEnvOrFail("DISCORD_GUILD_ID")
    googleFactcheckAPIKey := getEnvOrFail("GOOGLE_FACTCHECK_API_KEY")
    openaiAPIKey := getEnvOrFail("OPENAI_API_KEY")
    discordWebhooks := os.Getenv("DISCORD_WEBHOOKS") // Optional

    fmt.Println("All required environment variables loaded successfully.")

    // You can pass these vars into your module initializers or store in a global config struct.

    // Placeholder for future logic
    fmt.Println("Ready for expansion: Discord, news polling, fact-check integration, etc.")
}
