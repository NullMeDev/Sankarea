package main

import (
    "fmt"
    "log"
    "os"
    "path/filepath"

    "github.com/bwmarrin/discordgo"
)

func getEnvOrFail(key string) string {
    v := os.Getenv(key)
    if v == "" {
        log.Fatalf("Missing required environment variable: %s", key)
    }
    return v
}

func main() {
    fmt.Println("Sankarea bot starting up...")

    // Check critical data/config files
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

    // Load secrets from environment
    discordBotToken := getEnvOrFail("DISCORD_BOT_TOKEN")
    discordChannelID := getEnvOrFail("DISCORD_CHANNEL_ID")

    // Optional: load others if you need them for future modules
    // claimBusterAPIKey := getEnvOrFail("CLAIMBUSTER_API_KEY")
    // discordAppID := getEnvOrFail("DISCORD_APPLICATION_ID")
    // discordGuildID := getEnvOrFail("DISCORD_GUILD_ID")
    // googleFactcheckAPIKey := getEnvOrFail("GOOGLE_FACTCHECK_API_KEY")
    // openaiAPIKey := getEnvOrFail("OPENAI_API_KEY")
    // discordWebhooks := os.Getenv("DISCORD_WEBHOOKS") // Optional

    // Connect to Discord
    dg, err := discordgo.New("Bot " + discordBotToken)
    if err != nil {
        log.Fatalf("Error creating Discord session: %v", err)
    }

    // Open a websocket connection to Discord and begin listening.
    err = dg.Open()
    if err != nil {
        log.Fatalf("Error opening connection to Discord: %v", err)
    }
    defer dg.Close()

    // Send a test message to your channel on startup
    msg, err := dg.ChannelMessageSend(discordChannelID, "🟢 Sankarea bot is online and running.")
    if err != nil {
        log.Fatalf("Failed to send startup message: %v", err)
    }
    fmt.Printf("Startup message sent. Message ID: %s\n", msg.ID)

    // Keep the bot running until killed
    fmt.Println("Sankarea bot is running. Press CTRL+C to exit.")
    select {}
}
