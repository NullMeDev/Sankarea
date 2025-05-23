// cmd/sankarea/bot.go
package main

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/bwmarrin/discordgo"
)

// Bot represents the main bot instance
type Bot struct {
    discord    *discordgo.Session
    scheduler  *Scheduler
    database   *Database
    logger     *Logger
    formatter  *Formatter
    dashboard  *Dashboard
    factChecker *FactChecker
    config     *BotConfig
    startTime  time.Time
    mutex      sync.RWMutex
}

// NewBot creates a new bot instance
func NewBot(config *BotConfig) (*Bot, error) {
    // Create Discord session
    discord, err := discordgo.New("Bot " + config.Token)
    if err != nil {
        return nil, fmt.Errorf("failed to create Discord session: %v", err)
    }

    // Initialize logger
    if err := InitLogger(); err != nil {
        return nil, fmt.Errorf("failed to initialize logger: %v", err)
    }

    // Initialize database
    db, err := NewDatabase(config.DatabasePath)
    if err != nil {
        return nil, fmt.Errorf("failed to initialize database: %v", err)
    }

    bot := &Bot{
        discord:     discord,
        scheduler:   NewScheduler(discord),
        database:    db,
        logger:      Logger(),
        formatter:   NewFormatter(),
        factChecker: NewFactChecker(),
        config:      config,
        startTime:   time.Now(),
    }

    // Initialize dashboard if enabled
    if config.DashboardEnabled {
        dashboard, err := NewDashboard()
        if err != nil {
            return nil, fmt.Errorf("failed to initialize dashboard: %v", err)
        }
        bot.dashboard = dashboard
    }

    return bot, nil
}

// Start initializes and starts the bot
func (b *Bot) Start() error {
    b.logger.Info("Starting Sankarea News Bot v%s", VERSION)

    // Add event handlers
    b.discord.AddHandler(b.handleReady)
    b.discord.AddHandler(b.handleMessageCreate)
    b.discord.AddHandler(b.handleInteractionCreate)

    // Open Discord connection
    if err := b.discord.Open(); err != nil {
        return fmt.Errorf("failed to open Discord connection: %v", err)
    }

    // Start scheduler
    if err := b.scheduler.Start(); err != nil {
        return fmt.Errorf("failed to start scheduler: %v", err)
    }

    // Start dashboard if enabled
    if b.dashboard != nil {
        go func() {
            if err := b.dashboard.Start(); err != nil {
                b.logger.Error("Dashboard error: %v", err)
            }
        }()
    }

    return nil
}

// Stop gracefully shuts down the bot
func (b *Bot) Stop() error {
    b.logger.Info("Stopping bot...")

    // Stop scheduler
    b.scheduler.Stop()

    // Stop dashboard if running
    if b.dashboard != nil {
        if err := b.dashboard.Stop(); err != nil {
            b.logger.Error("Failed to stop dashboard: %v", err)
        }
    }

    // Close database connection
    if err := b.database.Close(); err != nil {
        b.logger.Error("Failed to close database: %v", err)
    }

    // Close Discord connection
    return b.discord.Close()
}

// Discord event handlers

func (b *Bot) handleReady(s *discordgo.Session, r *discordgo.Ready) {
    b.logger.Info("Bot is ready! Logged in as %s#%s", r.User.Username, r.User.Discriminator)
    
    // Update bot status
    err := s.UpdateGameStatus(0, "Monitoring news | /help")
    if err != nil {
        b.logger.Error("Failed to update status: %v", err)
    }
}

func (b *Bot) handleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
    // Ignore messages from the bot itself
    if m.Author.ID == s.State.User.ID {
        return
    }

    // TODO: Implement message handling logic
}

func (b *Bot) handleInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
    // Handle slash commands
    if i.Type == discordgo.InteractionApplicationCommand {
        if err := b.handleCommand(s, i); err != nil {
            b.logger.Error("Command error: %v", err)
            // Send error response to user
            s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
                Type: discordgo.InteractionResponseChannelMessageWithSource,
                Data: &discordgo.InteractionResponseData{
                    Content: fmt.Sprintf("❌ Error: %v", err),
                    Flags:   discordgo.MessageFlagsEphemeral,
                },
            })
        }
    }
}

// handleCommand processes slash commands
func (b *Bot) handleCommand(s *discordgo.Session, i *discordgo.InteractionCreate) error {
    // Check permissions
    if !CheckCommandPermissions(s, i) {
        return fmt.Errorf("you don't have permission to use this command")
    }

    // Get command data
    data := i.ApplicationCommandData()

    // Handle different commands
    switch data.Name {
    case "ping":
        return b.handlePingCommand(s, i)
    case "news":
        return b.handleNewsCommand(s, i)
    case "digest":
        return b.handleDigestCommand(s, i)
    default:
        return fmt.Errorf("unknown command: %s", data.Name)
    }
}

// Command handlers

func (b *Bot) handlePingCommand(s *discordgo.Session, i *discordgo.InteractionCreate) error {
    latency := time.Since(b.startTime)
    return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseChannelMessageWithSource,
        Data: &discordgo.InteractionResponseData{
            Content: fmt.Sprintf("🏓 Pong! Latency: %s", latency.Round(time.Millisecond)),
        },
    })
}

func (b *Bot) handleNewsCommand(s *discordgo.Session, i *discordgo.InteractionCreate) error {
    // TODO: Implement news command
    return nil
}

func (b *Bot) handleDigestCommand(s *discordgo.Session, i *discordgo.InteractionCreate) error {
    // TODO: Implement digest command
    return nil
}

// Utility methods

func (b *Bot) GetUptime() time.Duration {
    return time.Since(b.startTime)
}
