package main

import (
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/bwmarrin/discordgo"
    "github.com/robfig/cron/v3"
)

var (
    dg                *discordgo.Session
    cronJob           *cron.Cron
    discordChannelID  string
    auditLogChannelID string
    discordGuildID    string
    discordOwnerID    string
    sources           []Source
    state             State
)

func main() {
    // Ensure config and state files exist
    EnsureRequiredFiles()

    // Load environment variables
    discordBotToken := GetEnvOrFail("DISCORD_BOT_TOKEN")
    discordGuildID = GetEnvOrFail("DISCORD_GUILD_ID")
    discordChannelID = GetEnvOrFail("DISCORD_CHANNEL_ID")

    // Load config, sources, and state
    var err error
    currentConfig, err = LoadConfig()
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }
    auditLogChannelID = currentConfig.AuditLogChannelID

    sources, err = LoadSources()
    if err != nil {
        log.Fatalf("Failed to load sources: %v", err)
    }

    state, err = LoadState()
    if err != nil {
        log.Printf("No state found or failed to load: %v. Starting fresh.", err)
        state = State{}
    }

    // Create Discord session
    dg, err = discordgo.New("Bot " + discordBotToken)
    if err != nil {
        log.Fatalf("Failed to create Discord session: %v", err)
    }

    // Retrieve guild owner ID for admin checks
    guild, err := dg.Guild(discordGuildID)
    if err == nil {
        discordOwnerID = guild.OwnerID
    } else {
        log.Printf("Failed to get guild info: %v", err)
    }

    // Register slash commands on guild
    RegisterCommands(dg, discordGuildID)

    // Add interaction handler
    dg.AddHandler(handleCommands)

    // Open Discord websocket connection
    err = dg.Open()
    if err != nil {
        log.Fatalf("Failed to open Discord connection: %v", err)
    }
    defer dg.Close()

    // Notify that bot is online
    _, err = dg.ChannelMessageSend(discordChannelID, "🟢 Sankarea bot is online and ready. Use /setnewsinterval to control posting frequency.")
    if err != nil {
        log.Printf("Failed to send startup message: %v", err)
    }

    // Setup and start cron scheduler
    cronJob = cron.New()

    // Schedule RSS feed posting
    ScheduleRSSPosting(cronJob, dg, discordChannelID, sources)

    // Schedule article posting every 2 hours
    ScheduleArticlePosting(cronJob, dg, discordChannelID, sources)

    cronJob.Start()

    // Handle graceful shutdown on interrupt signals
    stop := make(chan os.Signal, 1)
    signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
    fmt.Println("Sankarea bot is running. Press CTRL+C to exit.")
    <-stop
    fmt.Println("Received shutdown signal, closing...")

    cronJob.Stop()
    _ = SaveState(state)
}
