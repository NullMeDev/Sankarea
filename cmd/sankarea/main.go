package main

import (
    "fmt"
    "log"
    "os"

    "github.com/bwmarrin/discordgo"
)

func getEnvOrFail(key string) string {
    v := os.Getenv(key)
    if v == "" {
        log.Fatalf("Missing required environment variable: %s", key)
    }
    return v
}

func fileMustExist(path string) {
    if _, err := os.Stat(path); os.IsNotExist(err) {
        log.Fatalf("ERROR: Required file not found: %s", path)
    }
}

func main() {
    fmt.Println("Sankarea bot starting up...")

    // Check essential config/data files exist
    fileMustExist("config/config.json")
    fileMustExist("config/sources.yml")
    fileMustExist("data/state.json")

    // Load secrets from environment
    discordBotToken := getEnvOrFail("DISCORD_BOT_TOKEN")
    discordChannelID := getEnvOrFail("DISCORD_CHANNEL_ID")

    // Connect to Discord
    dg, err := discordgo.New("Bot " + discordBotToken)
    if err != nil {
        log.Fatalf("Error creating Discord session: %v", err)
    }

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

    // Keep the bot running
    fmt.Println("Sankarea bot is running. Press CTRL+C to exit.")
    select {}
}
