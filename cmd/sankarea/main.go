// cmd/sankarea/main.go
package main

import (
    "context"
    "database/sql"
    "flag"
    "fmt"
    "os"
    "os/signal"
    "sync"
    "syscall"
    "time"
    
    "github.com/bwmarrin/discordgo"
)

var (
    // Global variables
    cfg          *Config
    state        *State
    mutex        sync.RWMutex
    db           *sql.DB
    errorBuffer  *ErrorBuffer
    healthMonitor *HealthMonitor
)

func main() {
    // Parse command line flags
    configFile := flag.String("config", "config/config.json", "path to config file")
    sourcesFile := flag.String("sources", "config/sources.yml", "path to sources file")
    flag.Parse()

    // Initialize logging
    Logger().Printf("Starting Sankarea News Bot v%s", cfg.Version)

    // Load configuration
    if err := loadConfiguration(); err != nil {
        Logger().Printf("Failed to load configuration: %v", err)
        os.Exit(1)
    }

    // Initialize environment
    if err := InitializeEnvironment(); err != nil {
        Logger().Printf("Failed to initialize environment: %v", err)
        os.Exit(1)
    }

    // Initialize error buffer
    errorBuffer = NewErrorBuffer(100)

    // Initialize state
    var err error
    state, err = LoadState()
    if err != nil {
        Logger().Printf("Failed to load state: %v", err)
        os.Exit(1)
    }

    // Update startup time
    state.StartupTime = time.Now()
    state.Version = cfg.Version

    // Initialize health monitor
    healthMonitor = NewHealthMonitor()
    healthMonitor.StartPeriodicChecks(1 * time.Minute)

    // Create Discord session
    discord, err := discordgo.New("Bot " + cfg.BotToken)
    if err != nil {
        Logger().Printf("Error creating Discord session: %v", err)
        os.Exit(1)
    }

    // Register event handlers
    discord.AddHandler(messageCreate)
    discord.AddHandler(interactionCreate)

    // Initialize database if enabled
    if cfg.EnableDatabase {
        if err := initializeDatabase(); err != nil {
            Logger().Printf("Failed to initialize database: %v", err)
            os.Exit(1)
        }
        defer db.Close()
    }

    // Open Discord connection
    if err := discord.Open(); err != nil {
        Logger().Printf("Error opening Discord connection: %v", err)
        os.Exit(1)
    }
    defer discord.Close()

    // Start dashboard if enabled
    if cfg.EnableDashboard {
        if err := StartDashboard(); err != nil {
            Logger().Printf("Failed to start dashboard: %v", err)
            os.Exit(1)
        }
    }

    // Register commands
    if err := registerCommands(discord); err != nil {
        Logger().Printf("Failed to register commands: %v", err)
        os.Exit(1)
    }

    // Initialize news fetching
    if cfg.FetchNewsOnStartup {
        go func() {
            if err := fetchNews(discord); err != nil {
                Logger().Printf("Initial news fetch failed: %v", err)
            }
        }()
    }

    // Start news scheduler
    scheduler := NewScheduler()
    scheduler.Start()

    // Wait for shutdown signal
    Logger().Printf("Bot is now running. Press CTRL-C to exit.")
    sc := make(chan os.Signal, 1)
    signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
    <-sc

    // Graceful shutdown
    Logger().Println("Shutting down...")
    
    // Stop scheduler
    scheduler.Stop()

    // Stop health monitor
    healthMonitor.StopChecks()

    // Update shutdown time
    state.ShutdownTime = time.Now()
    if err := SaveState(state); err != nil {
        Logger().Printf("Failed to save state during shutdown: %v", err)
    }

    Logger().Println("Shutdown complete")
}

func loadConfiguration() error {
    // Load environment configuration
    cfg = LoadEnvConfig()
    
    // Validate configuration
    if err := ValidateEnvConfig(cfg); err != nil {
        return fmt.Errorf("invalid configuration: %w", err)
    }

    return nil
}

func initializeDatabase() error {
    // Database initialization code would go here
    return nil
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
    // Ignore messages from the bot itself
    if m.Author.ID == s.State.User.ID {
        return
    }

    // Handle direct messages
    if m.GuildID == "" {
        handleDirectMessage(s, m)
        return
    }

    // Handle guild messages if needed
}

func interactionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
    // Check if user has permission to use commands
    if !CheckCommandPermissions(s, i) {
        respondWithError(s, i, "You don't have permission to use this command")
        return
    }

    // Handle different interaction types
    switch i.Type {
    case discordgo.InteractionApplicationCommand:
        handleSlashCommand(s, i)
    case discordgo.InteractionMessageComponent:
        handleMessageComponent(s, i)
    }
}

func handleSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
    // Get command data
    data := i.ApplicationCommandData()

    // Route to appropriate handler
    switch data.Name {
    case "ping":
        handlePingCommand(s, i)
    case "status":
        handleStatusCommand(s, i)
    case "source":
        handleSourceCommand(s, i)
    case "admin":
        handleAdminCommand(s, i)
    default:
        respondWithError(s, i, "Unknown command")
    }
}

func handleMessageComponent(s *discordgo.Session, i *discordgo.InteractionCreate) {
    // Handle button clicks and select menus
    data := i.MessageComponentData()
    
    // Route to appropriate handler based on custom ID
    switch data.CustomID {
    // Add handlers for different component IDs
    default:
        respondWithError(s, i, "Unknown component interaction")
    }
}

func handleDirectMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
    // Handle direct messages to the bot
    // This could be used for user-specific settings or help commands
}
