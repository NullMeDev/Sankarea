package main

import (
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"

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
    discordAppID := getEnvOrFail("DISCORD_APPLICATION_ID")
    discordGuildID := getEnvOrFail("DISCORD_GUILD_ID")
    discordChannelID := getEnvOrFail("DISCORD_CHANNEL_ID")

    // Connect to Discord
    dg, err := discordgo.New("Bot " + discordBotToken)
    if err != nil {
        log.Fatalf("Error creating Discord session: %v", err)
    }

    // Add interaction handler
    dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
        if i.Type == discordgo.InteractionApplicationCommand {
            switch i.ApplicationCommandData().Name {
            case "ping":
                s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
                    Type: discordgo.InteractionResponseChannelMessageWithSource,
                    Data: &discordgo.InteractionResponseData{
                        Content: "Pong!",
                    },
                })
            }
        }
    })

    // Open a websocket connection to Discord
    err = dg.Open()
    if err != nil {
        log.Fatalf("Error opening connection to Discord: %v", err)
    }
    defer dg.Close()

    // Register the /ping command for your guild
    _, err = dg.ApplicationCommandCreate(discordAppID, discordGuildID, &discordgo.ApplicationCommand{
        Name:        "ping",
        Description: "Test if the bot is alive",
    })
    if err != nil {
        log.Fatalf("Cannot create slash command: %v", err)
    }

    // Send a test message on startup
    _, err = dg.ChannelMessageSend(discordChannelID, "🟢 Sankarea bot is online and ready. Use `/ping` to test me.")
    if err != nil {
        log.Printf("Failed to send startup message: %v", err)
    }

    fmt.Println("Sankarea bot is running. Press CTRL+C to exit.")

    // Wait for a CTRL+C or kill signal
    stop := make(chan os.Signal, 1)
    signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
    <-stop

    fmt.Println("Sankarea bot shutting down...")
}
