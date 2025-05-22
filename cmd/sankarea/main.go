package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/robfig/cron/v3"
)

var (
	cfg             *Config
	dg              *discordgo.Session
	cronManager     *cron.Cron
	configManager   *ConfigManager
	ctx             context.Context
	cancel          context.CancelFunc
	imageDownloader *ImageDownloader
	filterManager   *UserFilterManager
	healthMonitor   *HealthMonitor
	keywordTracker  *KeywordTracker
	digester        *DigestManager
	langManager     *LanguageManager
	credScorer      *CredibilityScorer
	analyticsEngine *AnalyticsEngine
	errorSystem     *ErrorSystem
	botStartTime    time.Time
)

func main() {
	// Record startup time
	botStartTime = time.Now()

	// Create a cancellable context for the entire app
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	// Set up panic recovery
	defer RecoverFromPanic("main")

	fmt.Println("Sankarea bot v" + VERSION + " starting up...")

	// Initialize subsystems
	LoadEnv()
	EnsureDirectories()
	FileMustExist("config/config.json")
	FileMustExist("config/sources.yml")

	if err := SetupLogging(); err != nil {
		log.Printf("Warning: %v", err)
	}

	// Initialize error handling system
	errorSystem = NewErrorSystem(100) // Store last 100 errors

	// Load configuration
	var err error
	cfg, err = LoadConfig()
	if err != nil {
		errorSystem.HandleError("Failed to load config", err, "startup", ErrorSeverityFatal)
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
		errorSystem.HandleError("Failed to load sources", err, "startup", ErrorSeverityFatal)
	}

	state, err := LoadState()
	if err != nil {
		errorSystem.HandleError("Failed to load state", err, "startup", ErrorSeverityFatal)
	}

	// Initialize state with current time
	state.StartupTime = time.Now()
	if state.Version == "" {
		state.Version = cfg.Version
	}
	if err := SaveState(state); err != nil {
		errorSystem.HandleError("Failed to save initial state", err, "startup", ErrorSeverityMedium)
	}

	// Initialize subsystems
	initializeSubsystems()

	// Initialize discord connection
	dg, err = discordgo.New("Bot " + cfg.BotToken)
	if err != nil {
		errorSystem.HandleError("Failed to create Discord session", err, "discord", ErrorSeverityFatal)
	}

	// Set intents for increased functionality
	dg.Identify.Intents = discordgo.IntentsGuildMessages | 
		discordgo.IntentsGuildMessageReactions | 
		discordgo.IntentsDirectMessages |
		discordgo.IntentsMessageContent

	// Register handlers
	dg.AddHandler(handleReady)
	dg.AddHandler(handleInteraction)
	dg.AddHandler(handleMessage)

	// Start the bot connection
	err = dg.Open()
	if err != nil {
		errorSystem.HandleError("Failed to connect to Discord", err, "discord", ErrorSeverityFatal)
	}
	defer dg.Close()

	// Register slash commands
	registerCommands()

	// Start the dashboard
	if cfg.EnableDashboard {
		StartDashboard()
	}

	// Start news update cron
	startNewsUpdateCron()

	// Start digest scheduler
	digester.StartScheduler(dg)

	// Wait for termination signal
	Logger().Println("Bot is now running. Press CTRL+C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM)
	<-sc

	// Clean shutdown
	Logger().Println("Shutting down gracefully...")
	saveAllData()
	cancel() // Cancel all ongoing operations
}

func initializeSubsystems() {
	// Initialize image downloader
	imageDownloader = NewImageDownloader()
	if err := imageDownloader.Initialize(); err != nil {
		Logger().Printf("Warning: Image downloader initialization failed: %v", err)
	}

	// Initialize user filter manager
	filterManager = NewUserFilterManager()
	if err := filterManager.Initialize(); err != nil {
		Logger().Printf("Warning: User filter manager initialization failed: %v", err)
	}

	// Initialize health monitoring
	healthMonitor = NewHealthMonitor()
	healthMonitor.StartPeriodicChecks(5 * time.Minute)
	
	// Initialize keyword tracking
	keywordTracker = NewKeywordTracker()
	if err := keywordTracker.Initialize(); err != nil {
		Logger().Printf("Warning: Keyword tracker initialization failed: %v", err)
	}
	
	// Initialize digest manager
	digester = NewDigestManager()
	
	// Initialize language manager
	langManager = NewLanguageManager()
	if err := langManager.Initialize(); err != nil {
		Logger().Printf("Warning: Language manager initialization failed: %v", err)
	}
	
	// Initialize credibility scorer
	credScorer = NewCredibilityScorer()
	if err := credScorer.Initialize(); err != nil {
		Logger().Printf("Warning: Credibility scorer initialization failed: %v", err)
	}
	
	// Initialize analytics engine
	analyticsEngine = NewAnalyticsEngine()
	if err := analyticsEngine.Initialize(); err != nil {
		Logger().Printf("Warning: Analytics engine initialization failed: %v", err)
	}
	
	// Initialize database if enabled
	if cfg.EnableDatabase {
		if err := InitDB(); err != nil {
			errorSystem.HandleError("Failed to initialize database", err, "database", ErrorSeverityMedium)
			// Continue without database
		} else {
			Logger().Println("Database initialized successfully")
		}
	}
	
	// Start health API server if port is configured
	if cfg.HealthAPIPort > 0 {
		StartHealthServer(cfg.HealthAPIPort)
	}
}

func EnsureDirectories() {
	dirs := []string{
		"data",
		"logs",
		"config",
		"data/images",
		"data/user_filters",
		"data/analytics",
		"data/cache",
		"dashboard/templates",
		"dashboard/static",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}
}

func handleReady(s *discordgo.Session, r *discordgo.Ready) {
	Logger().Printf("Logged in as: %s#%s", r.User.Username, r.User.Discriminator)
	
	// Set bot status
	err := s.UpdateGameStatus(0, "Monitoring news | /help")
	if err != nil {
		Logger().Printf("Error setting status: %v", err)
	}
	
	// Log guilds
	Logger().Printf("Connected to %d guilds", len(r.Guilds))
	
	// Handle first-time setup if needed
	if len(r.Guilds) > 0 && cfg.GuildID == "" {
		cfg.GuildID = r.Guilds[0].ID
		Logger().Printf("Setting default guild ID to %s", cfg.GuildID)
		SaveConfig(cfg)
	}
	
	// Run an immediate news fetch if enabled
	if cfg.FetchNewsOnStartup {
		Logger().Println("Performing initial news fetch...")
		go fetchAllNews(s)
	}
}

func startNewsUpdateCron() {
	cronManager = cron.New()
	
	// Schedule news updates based on configuration
	if cfg.News15MinCron != "" {
		_, err := cronManager.AddFunc(cfg.News15MinCron, func() {
			fetchAllNews(dg)
		})
		
		if err != nil {
			Logger().Printf("Error scheduling news updates: %v", err)
		} else {
			Logger().Printf("Scheduled news updates with cron: %s", cfg.News15MinCron)
		}
	} else {
		// Default to every 15 minutes if not specified
		_, err := cronManager.AddFunc("*/15 * * * *", func() {
			fetchAllNews(dg)
		})
		
		if err != nil {
			Logger().Printf("Error scheduling default news updates: %v", err)
		} else {
			Logger().Println("Scheduled default news updates every 15 minutes")
		}
	}
	
	// Start cron manager
	cronManager.Start()
}

func fetchAllNews(s *discordgo.Session) {
	sources, err := LoadSources()
	if err != nil {
		Logger().Printf("Error loading sources: %v", err)
		return
	}
	
	state, err := LoadState()
	if err != nil {
		Logger().Printf("Error loading state: %v", err)
		return
	}
	
	if state.Paused {
		Logger().Println("News fetching is currently paused")
		return
	}
	
	fetchAndPostNews(s, cfg.NewsChannelID, sources)
	
	// Update state
	state.LastFetchTime = time.Now()
	SaveState(state)
}

func registerCommands() {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "ping",
			Description: "Check if the bot is online",
		},
		{
			Name:        "status",
			Description: "Shows the current status of the bot",
		},
		{
			Name:        "version",
			Description: "Shows the current version of the bot",
		},
		{
			Name:        "source",
			Description: "Manage RSS sources",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "list",
					Description: "List all RSS sources",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "add",
					Description: "Add a new RSS source",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "name",
							Description: "Name of the source",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "url",
							Description: "URL of the RSS feed",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "category",
							Description: "Category of the source",
							Required:    false,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "remove",
					Description: "Remove an RSS source",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "name",
							Description: "Name of the source to remove",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "info",
					Description: "Show details about a source",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "name",
							Description: "Name of the source",
							Required:    true,
						},
					},
				},
			},
		},
		{
			Name:        "admin",
			Description: "Admin commands",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "pause",
					Description: "Pause news updates",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "resume",
					Description: "Resume news updates",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "refresh",
					Description: "Force refresh of all news sources",
				},
			},
		},
		{
			Name:        "factcheck",
			Description: "Fact check a claim or article",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "claim",
					Description: "The claim to fact check",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "url",
					Description: "URL to an article (optional)",
					Required:    false,
				},
			},
		},
		{
			Name:        "summarize",
			Description: "Summarize an article",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "url",
					Description: "URL to the article",
					Required:    true,
				},
			},
		},
		{
			Name:        "help",
			Description: "Shows help information",
		},
		{
			Name:        "filter",
			Description: "Set your news filtering preferences",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "show",
					Description: "Show your current filter settings",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "source",
					Description: "Enable/disable a specific source",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "name",
							Description: "Name of the source",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionBoolean,
							Name:        "enabled",
							Description: "Whether to enable or disable this source",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "category",
					Description: "Enable/disable a news category",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "category",
							Description: "Name of the category",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionBoolean,
							Name:        "enabled",
							Description: "Whether to enable or disable this category",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "keywords",
					Description: "Set keywords to include or exclude",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "include",
							Description: "Keywords to include (comma separated)",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "exclude",
							Description: "Keywords to exclude (comma separated)",
							Required:    false,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "reset",
					Description: "Reset all your filter preferences",
				},
			},
		},
		{
			Name:        "suggest",
			Description: "Suggest a new news source",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "url",
					Description: "URL of the RSS feed",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "name",
					Description: "Name of the source",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "description",
					Description: "Description of the source",
					Required:    false,
				},
			},
		},
		{
			Name:        "digest",
			Description: "Generate and customize news digests",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "now",
					Description: "Generate a news digest right now",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionInteger,
							Name:        "stories",
							Description: "Maximum number of stories to include",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "categories",
							Description: "Categories to include (comma separated)",
							Required:    false,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "settings",
					Description: "Configure your digest settings",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionBoolean,
							Name:        "enabled",
							Description: "Enable or disable digests",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "schedule",
							Description: "Schedule in cron format (e.g. '0 8 * * *' for 8 AM daily)",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionInteger,
							Name:        "max_stories",
							Description: "Maximum number of stories to include",
							Required:    false,
						},
					},
				},
			},
		},
		{
			Name:        "track",
			Description: "Track keywords in news articles",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "add",
					Description: "Add a keyword to track",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "keyword",
							Description: "Keyword or phrase to track",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "remove",
					Description: "Remove a tracked keyword",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "keyword",
							Description: "Keyword to remove from tracking",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "list",
					Description: "List currently tracked keywords",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "stats",
					Description: "Show statistics for a tracked keyword",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "keyword",
							Description: "Keyword to show statistics for",
							Required:    true,
						},
					},
				},
			},
		},
		{
			Name:        "language",
			Description: "Change your language settings",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "set",
					Description: "Set your preferred language",
					Required:    true,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "English",
							Value: "en",
						},
						{
							Name:  "Spanish",
							Value: "es",
						},
						{
							Name:  "French",
							Value: "fr",
						},
						{
							Name:  "German",
							Value: "de",
						},
						{
							Name:  "Japanese",
							Value: "ja",
						},
					},
				},
			},
		},
	}

	// Register commands
	if cfg.GuildID != "" {
		// Guild commands update instantly
		for _, cmd := range commands {
			_, err := dg.ApplicationCommandCreate(cfg.AppID, cfg.GuildID, cmd)
			if err != nil {
				Logger().Printf("Error creating command %s: %v", cmd.Name, err)
			}
		}
	} else {
		// Global commands (can take up to an hour to propagate)
		for _, cmd := range commands {
			_, err := dg.ApplicationCommandCreate(cfg.AppID, "", cmd)
			if err != nil {
				Logger().Printf("Error creating global command %s: %v", cmd.Name, err)
			}
		}
	}
}

func handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages from the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}
	
	// Process direct messages specially
	if isDM(s, m) {
		handleDirectMessage(s, m)
		return
	}
	
	// Process guild messages
	if strings.HasPrefix(m.Content, "!news") {
		handleLegacyCommand(s, m)
	}
	
	// Track any keywords in the message
	if keywordTracker != nil {
		go keywordTracker.CheckForKeywords(m.Content)
	}
}

func isDM(s *discordgo.Session, m *discordgo.MessageCreate) bool {
	channel, err := s.Channel(m.ChannelID)
	if err != nil {
		return false
	}
	
	return channel.Type == discordgo.ChannelTypeDM
}

func handleDirectMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Simple DM handling
	if strings.HasPrefix(m.Content, "help") {
		s.ChannelMessageSend(m.ChannelID, "You can use slash commands like /help in servers where I'm added. Direct message support is limited.")
	}
}

func handleLegacyCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Handle legacy text commands for backward compatibility
	s.ChannelMessageSend(m.ChannelID, "Please use slash commands instead. Try /help for more information.")
}

func saveAllData() {
	Logger().Println("Saving all data before shutdown...")

	// Save current state
	state, err := LoadState()
	if err != nil {
		Logger().Printf("Error loading state for save: %v", err)
	} else {
		state.ShutdownTime = time.Now()
		SaveState(state)
	}

	// Save any pending data from subsystems
	if keywordTracker != nil {
		keywordTracker.Save()
	}

	if analyticsEngine != nil {
		analyticsEngine.Save()
	}

	if credScorer != nil {
		credScorer.Save()
	}

	Logger().Println("Data saved successfully")
}

// RecoverFromPanic handles panics gracefully
func RecoverFromPanic(component string) {
	if r := recover(); r != nil {
		Logger().Printf("PANIC RECOVERED in %s: %v", component, r)
		errorSystem.HandleError(
			fmt.Sprintf("Panic in %s", component),
			fmt.Errorf("%v", r),
			component,
			ErrorSeverityHigh,
		)
	}
}
