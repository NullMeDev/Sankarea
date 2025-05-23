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

// Discord event handlers

// handleReady handles the ready event when the bot connects to Discord
func (b *Bot) handleReady(s *discordgo.Session, r *discordgo.Ready) {
    b.logger.Info("Bot is ready! Logged in as %s#%s", s.State.User.Username, s.State.User.Discriminator)
    
    // Set custom status
    err := s.UpdateGameStatus(0, "Monitoring news feeds")
    if err != nil {
        b.logger.Error("Failed to set status: %v", err)
    }
}

// handleMessageCreate handles new message events
func (b *Bot) handleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
    // Ignore messages from the bot itself
    if m.Author.ID == s.State.User.ID {
        return
    }

    // Check if message starts with command prefix
    if !strings.HasPrefix(m.Content, "!") {
        return
    }

    // Split message into command and arguments
    args := strings.Fields(m.Content)
    if len(args) == 0 {
        return
    }

    // Extract command
    cmd := strings.ToLower(strings.TrimPrefix(args[0], "!"))

    // Handle commands
    switch cmd {
    case "sources":
        b.handleSourcesCommand(s, m)
    case "status":
        b.handleStatusCommand(s, m)
    case "help":
        b.handleHelpCommand(s, m)
    case "refresh":
        // Only allow owner to use refresh command
        if m.Author.ID == b.config.OwnerID {
            b.handleRefreshCommand(s, m)
        }
    }
}

// handleInteractionCreate handles slash command interactions
func (b *Bot) handleInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
    // Handle different interaction types
    switch i.Type {
    case discordgo.InteractionApplicationCommand:
        b.handleSlashCommand(s, i)
    case discordgo.InteractionMessageComponent:
        b.handleMessageComponent(s, i)
    }
}

// Command handlers

func (b *Bot) handleSourcesCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
    // Create categories map
    categories := make(map[string][]string)
    
    // Group sources by category
    for _, source := range b.scheduler.GetSources() {
        categories[source.Category] = append(categories[source.Category], source.Name)
    }

    // Create embed
    embed := &discordgo.MessageEmbed{
        Title: "Available News Sources",
        Color: 0x7289DA,
        Fields: []*discordgo.MessageEmbedField{},
    }

    // Add fields for each category
    for category, sources := range categories {
        // Sort sources alphabetically
        sort.Strings(sources)
        
        embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
            Name:   category,
            Value:  strings.Join(sources, "\n"),
            Inline: false,
        })
    }

    // Send embed
    _, err := s.ChannelMessageSendEmbed(m.ChannelID, embed)
    if err != nil {
        b.logger.Error("Failed to send sources list: %v", err)
    }
}

func (b *Bot) handleStatusCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
    // Get bot statistics
    stats := b.scheduler.GetStats()
    uptime := time.Since(b.startTime).Round(time.Second)

    // Create embed
    embed := &discordgo.MessageEmbed{
        Title: "Bot Status",
        Color: 0x43B581,
        Fields: []*discordgo.MessageEmbedField{
            {
                Name:   "Uptime",
                Value:  uptime.String(),
                Inline: true,
            },
            {
                Name:   "Articles Fetched",
                Value:  fmt.Sprintf("%d", stats.ArticleCount),
                Inline: true,
            },
            {
                Name:   "Active Sources",
                Value:  fmt.Sprintf("%d", stats.ActiveSources),
                Inline: true,
            },
            {
                Name:   "Last Update",
                Value:  stats.LastUpdate.Format("2006-01-02 15:04:05 MST"),
                Inline: false,
            },
        },
        Timestamp: time.Now().Format(time.RFC3339),
    }

    // Add error information if there are any
    if stats.LastError != "" {
        embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
            Name:   "Last Error",
            Value:  stats.LastError,
            Inline: false,
        })
        embed.Color = 0xF04747 // Red color for error state
    }

    // Send embed
    _, err := s.ChannelMessageSendEmbed(m.ChannelID, embed)
    if err != nil {
        b.logger.Error("Failed to send status: %v", err)
    }
}

func (b *Bot) handleHelpCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
    embed := &discordgo.MessageEmbed{
        Title: "Sankarea Bot Commands",
        Color: 0x7289DA,
        Fields: []*discordgo.MessageEmbedField{
            {
                Name:   "!sources",
                Value:  "List all available news sources by category",
                Inline: false,
            },
            {
                Name:   "!status",
                Value:  "Show bot status and statistics",
                Inline: false,
            },
            {
                Name:   "!help",
                Value:  "Show this help message",
                Inline: false,
            },
        },
    }

    // Add owner-only commands if message author is owner
    if m.Author.ID == b.config.OwnerID {
        embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
            Name:   "!refresh",
            Value:  "Force refresh of all news feeds (Owner only)",
            Inline: false,
        })
    }

    _, err := s.ChannelMessageSendEmbed(m.ChannelID, embed)
    if err != nil {
        b.logger.Error("Failed to send help message: %v", err)
    }
}

func (b *Bot) handleRefreshCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
    // Send acknowledgment
    msg, err := s.ChannelMessageSend(m.ChannelID, "🔄 Refreshing news feeds...")
    if err != nil {
        b.logger.Error("Failed to send refresh acknowledgment: %v", err)
        return
    }

    // Trigger refresh
    err = b.scheduler.RefreshNow()
    if err != nil {
        _, _ = s.ChannelMessageEdit(m.ChannelID, msg.ID, "❌ Failed to refresh feeds: "+err.Error())
        return
    }

    // Update message with success
    _, err = s.ChannelMessageEdit(m.ChannelID, msg.ID, "✅ News feeds refreshed successfully!")
    if err != nil {
        b.logger.Error("Failed to update refresh message: %v", err)
    }
}

func (b *Bot) handleSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
    // Get command name
    cmd := i.ApplicationCommandData().Name

    // Handle different commands
    switch cmd {
    case "sources":
        b.handleSourcesSlashCommand(s, i)
    case "status":
        b.handleStatusSlashCommand(s, i)
    default:
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: "Unknown command",
            },
        })
    }
}

func (b *Bot) handleMessageComponent(s *discordgo.Session, i *discordgo.InteractionCreate) {
    // Handle button clicks or select menus
    // This can be expanded later if needed
    s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseChannelMessageWithSource,
        Data: &discordgo.InteractionResponseData{
            Content: "Action acknowledged",
            Flags:   discordgo.MessageFlagsEphemeral,
        },
    })
}
