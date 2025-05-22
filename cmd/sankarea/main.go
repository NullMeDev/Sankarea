package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/robfig/cron/v3"
)

var (
	cfg           *Config
	dg            *discordgo.Session
	cronManager   *cron.Cron
	configManager *ConfigManager
	ctx           context.Context
	cancel        context.CancelFunc
)

func main() {
	// Create a cancellable context for the entire app
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	// Set up panic recovery
	defer RecoverFromPanic("main")

	fmt.Println("Sankarea bot v" + VERSION + " starting up...")

	// Initialize subsystems
	LoadEnv()
	FileMustExist("config/config.json")
	FileMustExist("config/sources.yml")
	EnsureDataDir()

	if err := SetupLogging(); err != nil {
		log.Printf("Warning: %v", err)
	}

	// Initialize error handling system
	InitErrorSystem(100) // Store last 100 errors

	// Load configuration
	var err error
	cfg, err = LoadConfig()
	if err != nil {
		HandleError("Failed to load config", err, "startup", ErrorSeverityFatal)
	}

	// Start config manager for auto-reload
	configManager, err = NewConfigManager(configFilePath, 1*time.Minute)
	if err != nil {
		Logger().Printf("Warning: Config auto-reload not available: %v", err)
	} else {
		configManager.SetReloadHandler(func(newCfg *Config) {
			cfg = newCfg
			Logger().Println("Configuration reloaded successfully")
		})
		configManager.StartWatching()
	}

	// Load sources and state
	sources, err := LoadSources()
	if err != nil {
		HandleError("Failed to load sources", err, "startup", ErrorSeverityFatal)
	}

	state, err := LoadState()
	if err != nil {
		HandleError("Failed to load state", err, "startup", ErrorSeverityFatal)
	}

	// Initialize state with current time
	state.StartupTime = time.Now()
	if state.Version == "" {
		state.Version = cfg.Version
	}
	if err := SaveState(state); err != nil {
		HandleError("Failed to save initial state", err, "startup", ErrorSeverityMedium)
	}

	// Initialize database if enabled
	if cfg.EnableDatabase {
		if err := InitDB(); err != nil {
			HandleError("Failed to initialize database", err, "database", ErrorSeverityMedium)
			// Continue without database
		} else {
			Logger().Println("Database initialized successfully")
		}
	}

	// Initialize health monitoring
	healthMonitor := InitHealthMonitor()
	RunPeriodicHealthChecks(5 * time.Minute)
	
	// Start health API server if port is configured
	if cfg.HealthAPIPort > 0 {
		StartHealthServer(cfg.HealthAPIPort)
	}

	// Initialize Discord connection
	dg, err = discordgo.New("Bot " + cfg.BotToken)
	if err != nil {
		HandleError("Failed to create Discord session", err, "discord", ErrorSeverityFatal)
	}

	// Register commands and handlers
	if err := RegisterCommands(dg, cfg.AppID, cfg.GuildID); err != nil {
		HandleError("Failed to register commands", err, "discord", ErrorSeverityFatal)
	}
	
	dg.AddHandler(handleInteraction)
	
	// Add intents for increased functionality
	dg.Identify.Intents = discordgo.IntentsGuildMessages | 
						  discordgo.IntentsGuildMessageReactions |
						  discordgo.IntentDirectMessages

	// Connect to Discord
	if err := dg.Open(); err != nil {
		HandleError("Failed to open Discord connection", err, "discord", ErrorSeverityFatal)
	}
	
	Logger().Println("Sankarea bot is now connected to Discord")

	// Update health status
	healthMonitor.UpdateHealthCheck("discord_connection", HealthStatusHealthy, "Connected to Discord gateway")

	// Send startup message if channel is configured
	if cfg.AuditLogChannelID != "" {
		startupMsg := fmt.Sprintf("🤖 Sankarea v%s started at <t:%d:F>", 
			cfg.Version, 
			time.Now().Unix())
		dg.ChannelMessageSend(cfg.AuditLogChannelID, startupMsg)
	}

	// Initialize news delivery system
	deliverySystem := NewNewsDeliverySystem(dg, cfg.NewsChannelID)
	
	// Add default channel configurations
	if cfg.DigestChannelID != "" && cfg.DigestChannelID != cfg.NewsChannelID {
		deliverySystem.AddChannelConfig(ChannelConfiguration{
			ChannelID:          cfg.DigestChannelID,
			MaxArticlesPerUpdate: 10,
			FormatStyle:        "detailed",
			UseSummaries:       true,
			UseFactChecking:    true,
		})
	}

	// Initialize scheduler
	cronManager = cron.New()
	var entryID cron.EntryID
	
	// Setup news fetch job
	UpdateCronJob(cronManager, &entryID, cfg.NewsIntervalMinutes, dg, sources, deliverySystem)
	
	// Setup digest job
	if cfg.DigestCronSchedule != "" {
		_, err = cronManager.AddFunc(cfg.DigestCronSchedule, func() {
			channelID := cfg.DigestChannelID
			if channelID == "" {
				channelID = cfg.NewsChannelID
			}
			
			err := GenerateDailyDigest(dg, channelID)
			if err != nil {
				HandleError("Failed to generate daily digest", err, "digest", ErrorSeverityMedium)
			}
		})
		
		if err != nil {
			HandleError("Failed to schedule digest job", err, "scheduler", ErrorSeverityMedium)
		} else {
			Logger().Printf("Scheduled digest for: %s", cfg.DigestCronSchedule)
		}
	}

	// Start admin dashboard if enabled
	if cfg.EnableDashboard {
		// Ensure dashboard templates exist
		if err := CreateDefaultDashboardTemplates(); err != nil {
			HandleError("Failed to create dashboard templates", err, "dashboard", ErrorSeverityLow)
		}
		
		StartDashboard()
	}

	// Start scheduler
	cronManager.Start()
	Logger().Println("Scheduler started successfully")

	// Wait for CTRL+C or SIGTERM
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	// Begin graceful shutdown
	Logger().Println("Shutdown signal received")
	
	// Cancel context to notify all goroutines
	cancel()

	// Send shutdown message if channel is configured
	if cfg.AuditLogChannelID != "" {
		shutdownMsg := fmt.Sprintf("🔌 Sankarea bot shutting down at <t:%d:F>", 
			time.Now().Unix())
		dg.ChannelMessageSend(cfg.AuditLogChannelID, shutdownMsg)
	}

	// Stop services in order
	if configManager != nil {
		configManager.Stop()
	}
	
	cronManager.Stop()
	Logger().Println("Scheduler stopped")
	
	// Close Discord connection
	dg.Close()
	Logger().Println("Discord connection closed")
	
	// Close database connection
	if db != nil {
		CloseDB()
		Logger().Println("Database connection closed")
	}
	
	Logger().Println("Sankarea bot shutdown complete")
}
