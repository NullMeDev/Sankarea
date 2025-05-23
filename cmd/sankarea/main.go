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
    cfg           *Config
    db            *sql.DB
    errorBuffer   *ErrorBuffer
    healthMonitor *HealthMonitor
    dashboard     *DashboardServer
    botVersion    = "1.0.0" // This should be updated with your actual version
)

func main() {
    startTime := time.Now()

    // Parse command line flags
    configFile := flag.String("config", "config/config.json", "path to config file")
    sourcesFile := flag.String("sources", "config/sources.yml", "path to sources file")
    logLevel := flag.String("log-level", "info", "logging level (debug, info, warn, error)")
    flag.Parse()

    // Initialize logging first
    if err := InitializeLogging(*logLevel); err != nil {
        fmt.Printf("Failed to initialize logging: %v\n", err)
        os.Exit(1)
    }

    Logger().Printf("Starting Sankarea News Bot v%s", botVersion)
    Logger().Printf("Started by %s at %s UTC", "NullMeDev", startTime.Format("2006-01-02 15:04:05"))

    // Load configuration
    if err := loadConfiguration(*configFile); err != nil {
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
    if err := InitializeState(); err != nil {
        Logger().Printf("Failed to initialize state: %v", err)
        os.Exit(1)
    }

    // Initialize health monitor
    healthMonitor = NewHealthMonitor()
    healthMonitor.StartPeriodicChecks(1 * time.Minute)
    defer healthMonitor.StopChecks()

    // Create Discord session
    discord, err := initializeDiscord()
    if err != nil {
        Logger().Printf("Error creating Discord session: %v", err)
        os.Exit(1)
    }
    defer discord.Close()

    // Initialize database if enabled
    if cfg.EnableDatabase {
        if err := initializeDatabase(); err != nil {
            Logger().Printf("Failed to initialize database: %v", err)
            os.Exit(1)
        }
        defer db.Close()
    }

    // Start dashboard if enabled
    if cfg.EnableDashboard {
        if err := initializeDashboard(); err != nil {
            Logger().Printf("Failed to start dashboard: %v", err)
            os.Exit(1)
        }
    }

    // Initialize scheduler
    scheduler := NewScheduler()
    scheduler.Start()
    defer scheduler.Stop()

    // Register commands
    if err := registerCommands(discord); err != nil {
        Logger().Printf("Failed to register commands: %v", err)
        os.Exit(1)
    }

    // Initial news fetch if configured
    if cfg.FetchNewsOnStartup {
        go func() {
            if err := fetchNews(discord); err != nil {
                Logger().Printf("Initial news fetch failed: %v", err)
            }
        }()
    }

    // Log startup completion
    Logger().Printf("Startup completed in %s", time.Since(startTime))
    Logger().Printf("Bot is now running. Press CTRL-C to exit.")

    // Wait for shutdown signal
    shutdownChan := make(chan os.Signal, 1)
    signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
    <-shutdownChan

    // Graceful shutdown
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    Logger().Println("Initiating graceful shutdown...")
    
    // Begin shutdown sequence
    if err := performGracefulShutdown(ctx, discord); err != nil {
        Logger().Printf("Error during shutdown: %v", err)
    }

    Logger().Println("Shutdown complete")
}

func initializeDiscord() (*discordgo.Session, error) {
    discord, err := discordgo.New("Bot " + cfg.BotToken)
    if err != nil {
        return nil, fmt.Errorf("error creating Discord session: %v", err)
    }

    // Register event handlers
    discord.AddHandler(messageCreate)
    discord.AddHandler(interactionCreate)
    discord.AddHandler(ready)

    // Open Discord connection
    if err := discord.Open(); err != nil {
        return nil, fmt.Errorf("error opening Discord connection: %v", err)
    }

    return discord, nil
}

func initializeDashboard() error {
    dashboard = NewDashboardServer()
    if err := dashboard.Initialize(); err != nil {
        return fmt.Errorf("failed to initialize dashboard: %v", err)
    }
    return dashboard.Start()
}

func InitializeState() error {
    var err error
    state, err = LoadState()
    if err != nil {
        return fmt.Errorf("failed to load state: %v", err)
    }

    // Update startup information
    return UpdateState(func(s *State) {
        s.StartupTime = time.Now()
        s.Version = botVersion
        s.ErrorCount = 0
        s.LastInterval = cfg.NewsIntervalMinutes
    })
}

func performGracefulShutdown(ctx context.Context, discord *discordgo.Session) error {
    // Update shutdown time
    if err := UpdateState(func(s *State) {
        s.ShutdownTime = time.Now()
    }); err != nil {
        Logger().Printf("Failed to update shutdown time: %v", err)
    }

    // Cleanup tasks
    var wg sync.WaitGroup
    errChan := make(chan error, 3)

    // Close Discord connection
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := discord.Close(); err != nil {
            errChan <- fmt.Errorf("discord shutdown error: %v", err)
        }
    }()

    // Close database if enabled
    if cfg.EnableDatabase && db != nil {
        wg.Add(1)
        go func() {
            defer wg.Done()
            if err := db.Close(); err != nil {
                errChan <- fmt.Errorf("database shutdown error: %v", err)
            }
        }()
    }

    // Save final state
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := SaveState(state); err != nil {
            errChan <- fmt.Errorf("state save error: %v", err)
        }
    }()

    // Wait for all cleanup tasks or context timeout
    done := make(chan struct{})
    go func() {
        wg.Wait()
        close(done)
    }()

    select {
    case <-ctx.Done():
        return fmt.Errorf("shutdown timed out")
    case err := <-errChan:
        return err
    case <-done:
        return nil
    }
}

func ready(s *discordgo.Session, r *discordgo.Ready) {
    Logger().Printf("Logged in as: %s#%s", s.State.User.Username, s.State.User.Discriminator)
    s.UpdateGameStatus(0, fmt.Sprintf("v%s | /help", botVersion))
}
