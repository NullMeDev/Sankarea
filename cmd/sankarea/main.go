// cmd/sankarea/main.go
package main

import (
    "fmt"
    "log"
    "os"
    "os/signal"
    "sync"
    "syscall"
    "time"

    "github.com/bwmarrin/discordgo"
    "github.com/robfig/cron/v3"
)

var (
    cfg    *Config
    state  *State
    cronManager *cron.Cron
    mutex  sync.RWMutex
)

func init() {
    // Initialize configuration
    cfg = LoadEnvConfig()
    if err := ValidateEnvConfig(cfg); err != nil {
        log.Fatalf("Configuration error: %v", err)
    }

    // Initialize state
    state = &State{
        StartupTime: time.Now(),
        Version:     cfg.Version,
    }

    // Initialize environment
    if err := InitializeEnvironment(); err != nil {
        log.Fatalf("Environment initialization error: %v", err)
    }

    // Initialize cron manager
    cronManager = cron.New(cron.WithSeconds())
}

func main() {
    // Set up logging
    logFile, err := os.OpenFile("data/logs/sankarea.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
    if err != nil {
        log.Fatalf("Failed to open log file: %v", err)
    }
    defer logFile.Close()
    log.SetOutput(logFile)

    // Create Discord session
    dg, err := discordgo.New("Bot " + cfg.BotToken)
    if err != nil {
        log.Fatalf("Error creating Discord session: %v", err)
    }

    // Set up intents
    dg.Identify.Intents = discordgo.IntentsGuildMessages |
        discordgo.IntentsGuildMembers |
        discordgo.IntentsGuildMessageReactions |
        discordgo.IntentsDirectMessages

    // Register handlers
    dg.AddHandler(messageCreate)
    dg.AddHandler(ready)
    dg.AddHandler(guildCreate)
    dg.AddHandler(interactionCreate)

    // Connect to Discord
    err = dg.Open()
    if err != nil {
        log.Fatalf("Error opening Discord connection: %v", err)
    }
    defer dg.Close()

    // Initialize components
    if err := initializeComponents(dg); err != nil {
        log.Fatalf("Failed to initialize components: %v", err)
    }

    // Start health monitoring
    healthMonitor = NewHealthMonitor()
    healthMonitor.StartPeriodicChecks(time.Minute)
    defer healthMonitor.StopChecks()

    // Start the dashboard if enabled
    if cfg.EnableDashboard {
        if err := StartDashboard(); err != nil {
            log.Printf("Failed to start dashboard: %v", err)
        }
    }

    // Start scheduled tasks
    setupScheduledTasks(dg)

    // Initial news fetch if enabled
    if cfg.FetchNewsOnStartup {
        go fetchAndPostArticles(dg, cfg.NewsChannelID, loadSources(), cfg.MaxPostsPerSource)
    }

    // Wait for shutdown signal
    log.Printf("Bot is now running. Press CTRL-C to exit.")
    sc := make(chan os.Signal, 1)
    signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
    <-sc

    // Cleanup
    log.Println("Shutting down...")
    cleanupAndExit(dg)
}

func initializeComponents(s *discordgo.Session) error {
    // Initialize database if enabled
    if cfg.EnableDatabase {
        if err := InitDB(); err != nil {
            return fmt.Errorf("failed to initialize database: %v", err)
        }
    }

    // Register commands
    if err := registerCommands(s); err != nil {
        return fmt.Errorf("failed to register commands: %v", err)
    }

    return nil
}

func setupScheduledTasks(s *discordgo.Session) {
    // Schedule news fetching
    if _, err := cronManager.AddFunc(cfg.News15MinCron, func() {
        defer RecoverFromPanic("news-fetch")
        fetchAndPostArticles(s, cfg.NewsChannelID, loadSources(), cfg.MaxPostsPerSource)
    }); err != nil {
        log.Printf("Failed to schedule news fetching: %v", err)
    }

    // Schedule digest creation
    if _, err := cronManager.AddFunc(cfg.DigestCronSchedule, func() {
        defer RecoverFromPanic("digest")
        createAndSendDigest(s, cfg.NewsChannelID)
    }); err != nil {
        log.Printf("Failed to schedule digest creation: %v", err)
    }

    // Start the cron manager
    cronManager.Start()
}

func cleanupAndExit(s *discordgo.Session) {
    // Stop cron jobs
    if cronManager != nil {
        cronManager.Stop()
    }

    // Update state
    mutex.Lock()
    state.ShutdownTime = time.Now()
    saveState(state)
    mutex.Unlock()

    // Close Discord connection
    s.Close()

    // Final log message
    log.Printf("Bot shutdown complete. Uptime: %s", FormatDuration(time.Since(state.StartupTime)))
}

func loadSources() []Source {
    // Implementation of source loading would go here
    return []Source{}
}

// Event handlers
func ready(s *discordgo.Session, r *discordgo.Ready) {
    log.Printf("Logged in as: %v#%v", s.State.User.Username, s.State.User.Discriminator)
    s.UpdateGameStatus(0, fmt.Sprintf("v%s | /help", cfg.Version))
}

func guildCreate(s *discordgo.Session, g *discordgo.GuildCreate) {
    log.Printf("Added to guild: %v", g.Name)
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
    // Ignore messages from the bot itself
    if m.Author.ID == s.State.User.ID {
        return
    }

    // Handle direct messages
    if m.GuildID == "" {
        handleDirectMessage(s, m)
    }
}

func interactionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
    // Ensure the command is allowed
    if !CheckCommandPermissions(s, i) {
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: "You don't have permission to use this command.",
                Flags:   discordgo.MessageFlagsEphemeral,
            },
        })
        return
    }

    // Handle the command
    if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
        h(s, i)
    }
}

func handleDirectMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
    // Implementation of direct message handling would go here
}
