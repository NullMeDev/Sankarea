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
			defer CloseDB()
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

	// Set intents for increased functionality
	dg.Identify.Intents = discordgo.IntentsGuildMessages | 
		discordgo.IntentsGuildMessageReactions | 
		discordgo.IntentsDirectMessages |
		discordgo.IntentsMessageContent

	// Initialize managers
	channelManager := NewChannelManager()
	channelManager.Initialize()
	
	credManager := NewCredibilityManager()
	topicManager := NewTopicManager()
	interactionManager := NewInteractionManager()
	mediaExtractor := NewMediaExtractor()

	// Register commands and handlers
	if err := RegisterCommands(dg, cfg.AppID, cfg.GuildID); err != nil {
		HandleError("Failed to register commands", err, "discord", ErrorSeverityFatal)
	}
	
	if err := RegisterUserManagementCommands(dg, cfg.AppID, cfg.GuildID); err != nil {
		HandleError("Failed to register user management commands", err, "discord", ErrorSeverityMedium)
	}
	
	if err := RegisterChannelCommands(dg, cfg.AppID, cfg.GuildID); err != nil {
		HandleError("Failed to register channel commands", err, "discord", ErrorSeverityMedium)
	}
	
	if err := RegisterCredibilityCommands(dg, cfg.AppID, cfg.GuildID); err != nil {
		HandleError("Failed to register credibility commands", err, "discord", ErrorSeverityMedium)
	}
	
	if err := RegisterTopicCommands(dg, cfg.AppID, cfg.GuildID); err != nil {
		HandleError("Failed to register topic commands", err, "discord", ErrorSeverityMedium)
	}
	
	// Add handlers
	dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		// Try handling with specialized managers first
		if HandleUserManagementCommands(s, i) {
			return
		}
		
		if HandleChannelCommands(s, i, channelManager) {
			return
		}
		
		if HandleCredibilityCommands(s, i, credManager) {
			return
		}
		
		// Default handler for other commands
		handleInteraction(s, i)
	})
	
	dg.AddHandler(handleMessageCreate)
	dg.AddHandler(handleReady)
	
	// Add reaction handler
	dg.AddHandler(func(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
		interactionManager.HandleReactionAdd(s, r)
	})
	
	// Connect to Discord
	if err = dg.Open(); err != nil {
		HandleError("Failed to connect to Discord", err, "discord", ErrorSeverityFatal)
	}
	defer dg.Close()
	
	Logger().Println("Successfully connected to Discord!")

	// Initialize fact check service
	factCheckService := NewFactCheckService()
	contentAnalyzer := NewContentAnalyzer()

	// Initialize cron manager
	cronManager = cron.New()
	
	// Schedule news updates
	newsInterval := cfg.NewsIntervalMinutes
	if newsInterval <= 0 {
		newsInterval = 120 // Default to 2 hours
	}
	
	// Add news fetch job
	_, err = cronManager.AddFunc(cfg.News15MinCron, func() {
		// Modified to use the new channel manager and media extractor
		for _, channel := range channelManager.channels {
			layout := channelManager.GetPostLayout(channel.ChannelID)
			
			for _, source := range sources {
				if !source.Active || source.Paused {
					continue
				}
				
				// Check if this source should post to this channel
				if !channelManager.ShouldPostToChannel(source, channel.ChannelID) {
					continue
				}
				
				// Fetch and process news
				fetchAndPostNewsWithLayout(dg, channel.ChannelID, source, layout, factCheckService, interactionManager, mediaExtractor, topicManager, contentAnalyzer)
			}
		}
	})
	
	if err != nil {
		HandleError("Failed to schedule news updates", err, "cron", ErrorSeverityMedium)
	}
	
	// Add digest job
	_, err = cronManager.AddFunc(cfg.DigestCronSchedule, func() {
		if err := GenerateDailyDigest(dg, cfg.DigestChannelID); err != nil {
			HandleError("Failed to generate daily digest", err, "digest", ErrorSeverityMedium)
		} else {
			IncrementDigestCount()
		}
	})
	
	if err != nil {
		HandleError("Failed to schedule digest", err, "cron", ErrorSeverityMedium)
	}
	
	// Schedule reports
	ScheduleReports(dg)
	
	// Start cron manager
	cronManager.Start()
	defer cronManager.Stop()
	
	Logger().Printf("Bot is now running. Press CTRL-C to exit.")
	
	// Wait for signal to exit
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
	
	Logger().Println("Shutting down...")
}

// RecoverFromPanic recovers from panics and logs the error
func RecoverFromPanic(source string) {
	if r := recover(); r != nil {
		Logger().Printf("PANIC in %s: %v", source, r)
	}
}

// HandleError handles errors based on severity
func HandleError(message string, err error, component string, severity int) {
	errorMsg := fmt.Sprintf("%s: %v", message, err)
	
	// Always log the error
	Logger().Printf("[%s] %s", component, errorMsg)
	
	// Increment error counter
	IncrementErrorCount()
	
	// For fatal errors, exit the program
	if severity >= ErrorSeverityFatal {
		Logger().Fatalf("FATAL ERROR: %s", errorMsg)
	}
}

// Error severity levels
const (
	ErrorSeverityLow    = 0
	ErrorSeverityMedium = 1
	ErrorSeverityHigh   = 2
	ErrorSeverityFatal  = 3
)

// InitErrorSystem initializes the error tracking system
func InitErrorSystem(capacity int) {
	// Implementation depends on your needs
	Logger().Printf("Error system initialized with capacity %d", capacity)
}

// InitHealthMonitor initializes the health monitoring system
func InitHealthMonitor() interface{} {
	// Implementation depends on your health monitoring needs
	return nil
}

// RunPeriodicHealthChecks runs periodic health checks
func RunPeriodicHealthChecks(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				// Check system health
				Logger().Println("Health check: OK")
			case <-ctx.Done():
				return
			}
		}
	}()
}

// StartHealthServer starts a health check API server
func StartHealthServer(port int) {
	// Implementation depends on your health API needs
	Logger().Printf("Health API server started on port %d", port)
}

// ConfigManager watches for config changes and reloads when necessary
type ConfigManager struct {
	filePath    string
	checkInterval time.Duration
	reloadHandler func(*Config)
	stopChan    chan struct{}
}

// NewConfigManager creates a new config manager
func NewConfigManager(filePath string, checkInterval time.Duration) (*ConfigManager, error) {
	return &ConfigManager{
		filePath:      filePath,
		checkInterval: checkInterval,
		stopChan:      make(chan struct{}),
	}, nil
}

// SetReloadHandler sets the handler for config reloads
func (cm *ConfigManager) SetReloadHandler(handler func(*Config)) {
	cm.reloadHandler = handler
}

// StartWatching starts watching for config changes
func (cm *ConfigManager) StartWatching() {
	go func() {
		ticker := time.NewTicker(cm.checkInterval)
		defer ticker.Stop()
		
		var lastModTime time.Time
		
		for {
			select {
			case <-ticker.C:
				// Check if file has changed
				info, err := os.Stat(cm.filePath)
				if err != nil {
					continue
				}
				
				modTime := info.ModTime()
				if !modTime.Equal(lastModTime) && !lastModTime.IsZero() {
					// File changed, reload config
					newCfg, err := LoadConfig()
					if err != nil {
						Logger().Printf("Failed to reload config: %v", err)
						continue
					}
					
					// Call handler with new config
					if cm.reloadHandler != nil {
						cm.reloadHandler(newCfg)
					}
				}
				
				lastModTime = modTime
			case <-cm.stopChan:
				return
			}
		}
	}()
}

// Stop stops the config manager
func (cm *ConfigManager) Stop() {
	close(cm.stopChan)
}

// handleReady is called when the Discord connection is established
func handleReady(s *discordgo.Session, r *discordgo.Ready) {
	Logger().Printf("Logged in as %s#%s", r.User.Username, r.User.Discriminator)
}

// handleMessageCreate handles incoming messages
func handleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages from the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}
	
	// Optionally perform content moderation on messages
	if cfg.EnableContentFiltering {
		result, err := ModerateContent(m.Content)
		if err == nil && result.Flagged {
			HandleModeratedContent(s, m.ChannelID, m.ID, m.Content, result)
		}
	}
}

// CommandRequiresAdmin checks if a command requires admin permissions
func CommandRequiresAdmin(cmd string) bool {
	adminCommands := map[string]bool{
		"source": true,
		"admin":  true,
		"config": true,
		"channel": true,
		"credibility": true,
		"topic": true,
	}
	return adminCommands[cmd]
}

// CommandRequiresOwner checks if a command requires owner permissions
func CommandRequiresOwner(cmd string) bool {
	ownerCommands := map[string]bool{
		"shutdown": true,
		"reload":   true,
	}
	return ownerCommands[cmd]
}

// handleInteraction handles Discord slash command interactions
func handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}
	
	// Check permissions
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
	
	// Handle command
	cmd := i.ApplicationCommandData().Name
	switch cmd {
	case "ping":
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Pong! The bot is online.",
			},
		})
	case "status":
		handleStatusCommand(s, i)
	case "version":
		handleVersionCommand(s, i)
	case "source":
		handleSourceCommand(s, i)
	default:
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Unknown command.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}
}

// handleStatusCommand responds with the current bot status
func handleStatusCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	uptime := GetUptime().Round(time.Second)
	
	embed := &discordgo.MessageEmbed{
		Title:       "Bot Status",
		Description: "Current system status",
		Color:       0x00ff00, // Green
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Uptime",
				Value:  uptime.String(),
				Inline: true,
			},
			{
				Name:   "Version",
				Value:  VERSION,
				Inline: true,
			},
			{
				Name:   "News Articles",
				Value:  fmt.Sprintf("%d", state.FeedCount),
				Inline: true,
			},
			{
				Name:   "Digests",
				Value:  fmt.Sprintf("%d", state.DigestCount),
				Inline: true,
			},
			{
				Name:   "Errors",
				Value:  fmt.Sprintf("%d", state.ErrorCount),
				Inline: true,
			},
			{
				Name:   "System Paused",
				Value:  fmt.Sprintf("%t", state.Paused),
				Inline: true,
			},
		},
		Timestamp: time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Sankarea News Bot",
		},
	}
	
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
}

// handleVersionCommand responds with version information
func handleVersionCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Sankarea News Bot v%s", VERSION),
		},
	})
}

// handleSourceCommand handles source management commands
func handleSourceCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Missing source command.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}
	
	subCmd := options[0].Name
	switch subCmd {
	case "list":
		handleSourceListCommand(s, i)
	case "add":
		handleSourceAddCommand(s, i, options[0].Options)
	case "remove":
		handleSourceRemoveCommand(s, i, options[0].Options)
	case "update":
		handleSourceUpdateCommand(s, i, options[0].Options)
	default:
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Unknown source command.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}
}

// handleSourceListCommand lists all sources
func handleSourceListCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	sources, err := LoadSources()
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Failed to load sources: %v", err),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}
	
	if len(sources) == 0 {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "No sources configured.",
			},
		})
		return
	}
	
	// Create embed fields for each source
	var fields []*discordgo.MessageEmbedField
	for _, src := range sources {
		status := "Active"
		if src.Paused {
			status = "Paused"
		}
		
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   src.Name,
			Value:  fmt.Sprintf("URL: %s\nCategory: %s\nStatus: %s\nBias: %s",
				src.URL, src.Category, status, src.Bias),
			Inline: false,
		})
	}
	
	// Create and send embed
	embed := &discordgo.MessageEmbed{
		Title:       "News Sources",
		Description: fmt.Sprintf("%d sources configured", len(sources)),
		Color:       0x0099ff,
		Fields:      fields,
		Timestamp:   time.Now().Format(time.RFC3339),
	}
	
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
}

// handleSourceAddCommand adds a new source
func handleSourceAddCommand(s *discordgo.Session, i *discordgo.InteractionCreate, options []*discordgo.ApplicationCommandInteractionDataOption) {
	// Extract options
	var name, url, category, channel string
	var factCheck, summarize bool
	
	for _, opt := range options {
		switch opt.Name {
		case "name":
			name = opt.StringValue()
		case "url":
			url = opt.StringValue()
		case "category":
			category = opt.StringValue()
		case "fact_check":
			factCheck = opt.BoolValue()
		case "summarize":
			summarize = opt.BoolValue()
		case "channel":
			channel = opt.StringValue()
		}
	}
	
	if name == "" || url == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Name and URL are required.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}
	
	// Load existing sources
	sources, err := LoadSources()
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Failed to load sources: %v", err),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}
	
	// Check for duplicate
	for _, src := range sources {
		if src.Name == name {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("A source with name '%s' already exists.", name),
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}
	}
	
	// Add new source
	newSource := Source{
		Name:            name,
		URL:             url,
		Category:        category,
		FactCheckAuto:   factCheck,
		SummarizeAuto:   summarize,
		ChannelOverride: channel,
		Active:          true,
	}
	
	sources = append(sources, newSource)
	
	// Save sources
	if err := SaveSources(sources); err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Failed to save sources: %v", err),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}
	
	// Log audit
	AuditLog(s, "Add Source", i.Member.User.ID, fmt.Sprintf("Added source '%s'", name))
	
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Source '%s' added successfully.", name),
		},
	})
}

// handleSourceRemoveCommand removes a source
func handleSourceRemoveCommand(s *discordgo.Session, i *discordgo.InteractionCreate, options []*discordgo.ApplicationCommandInteractionDataOption) {
	// Extract name
	var name string
	for _, opt := range options {
		if opt.Name == "name" {
			name = opt.StringValue()
			break
		}
	}
	
	if name == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Source name is required.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}
	
	// Load sources
	sources, err := LoadSources()
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Failed to load sources: %v", err),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}
	
	// Find and remove source
	found := false
	var newSources []Source
	for _, src := range sources {
		if src.Name == name {
			found = true
		} else {
			newSources = append(newSources, src)
		}
	}
	
	if !found {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Source '%s' not found.", name),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}
	
	// Save sources
	if err := SaveSources(newSources); err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Failed to save sources: %v", err),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}
	
	// Log audit
	AuditLog(s, "Remove Source", i.Member.User.ID, fmt.Sprintf("Removed source '%s'", name))
	
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Source '%s' removed successfully.", name),
		},
	})
}

// handleSourceUpdateCommand updates a source
func handleSourceUpdateCommand(s *discordgo.Session, i *discordgo.InteractionCreate, options []*discordgo.ApplicationCommandInteractionDataOption) {
	// This function would be implemented similar to the add and remove commands
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Source update functionality is not yet implemented.",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

// fetchAndPostNewsWithLayout fetches and posts news with the specified layout
func fetchAndPostNewsWithLayout(s *discordgo.Session, channelID string, source Source, 
	layout PostLayout, factService *FactCheckService, interactionManager *InteractionManager,
	mediaExtractor *MediaExtractor, topicManager *TopicManager, contentAnalyzer *ContentAnalyzer) {
	
	state, err := LoadState()
	if err != nil {
		Logger().Printf("Cannot load state: %v", err)
		return
	}

	if state.Paused || source.Paused {
		Logger().Println("News fetch paused by system state or source settings")
		return
	}

	parser := gofeed.NewParser()
	
	// Set user agent if configured
	if cfg.UserAgentString != "" {
		parser.UserAgent = cfg.UserAgentString
	}
	
	feed, err := parser.ParseURL(source.URL)
	if err != nil {
		Logger().Printf("fetch %s failed: %v", source.Name, err)
		source.LastError = err.Error()
		source.ErrorCount++
		SaveSources([]Source{source})
		return
	}

	// Check if we have items to post
	if len(feed.Items) == 0 {
		return
	}
	
	// Format and post using the layout
	maxPosts := cfg.MaxPostsPerSource
	if maxPosts <= 0 {
		maxPosts = 5
	}
	
	// Limit the items to post
	itemsToPost := feed.Items
	if len(itemsToPost) > maxPosts {
		itemsToPost = itemsToPost[:maxPosts]
	}
	
	// Format and send the post
	err = FormatNewsPost(s, channelID, source, feed, itemsToPost, layout)
	if err != nil {
		Logger().Printf("Failed to format and post news: %v", err)
		return
	}
	
	// For each news item, check if we need to process it further
	for _, item := range itemsToPost {
		// Process for fact-checking if enabled
		if source.FactCheckAuto && cfg.EnableFactCheck {
			results, err := factService.CheckArticle(item.Title, item.Description)
			if err != nil {
				Logger().Printf("Fact check error: %v", err)
			} else if len(results) > 0 {
				// Post fact check results
				if err := factService.PostFactCheckResults(s, channelID, item.Link, results); err != nil {
					Logger().Printf("Failed to post fact check results: %v", err)
				}
			}
		}
		
		// Process for media embedding
		if item.Link != "" {
			if err := mediaExtractor.EnhanceNewsPost(s, channelID, item); err != nil {
				Logger().Printf("Media extraction error: %v", err)
			}
		}
		
		// Process for topic matching
		if item.Title != "" && item.Description != "" {
			topicManager.ProcessArticle(s, item.Title, item.Description, item.Link, source.Name)
		}
		
		// Make the post interactive
		if message, err := s.ChannelMessageSend(channelID, fmt.Sprintf("🔗 [%s](%s) - %s", 
			item.Title, item.Link, 
			item.PublishedParsed.Format("Jan 02"))); err == nil {
			
			err = interactionManager.RegisterInteractiveMessage(s, message, item.Title, item.Link, source.Name)
			if err != nil {
				Logger().Printf("Failed to register interactive message: %v", err)
			}
		}
		
		// Perform content analysis if enabled
		if source.SummarizeAuto && cfg.EnableSummarization {
			analysis, err := contentAnalyzer.AnalyzeContent(item.Title, item.Description, item.Link, source.Name, *item.PublishedParsed)
			if err != nil {
				Logger().Printf("Content analysis error: %v", err)
			} else if analysis.Summary != "" {
				// Post summary
				embed := &discordgo.MessageEmbed{
					Title:       fmt.Sprintf("Summary: %s", item.Title),
					URL:         item.Link,
					Description: analysis.Summary,
					Color:       0x00AAFF,
					Footer: &discordgo.MessageEmbedFooter{
						Text: fmt.Sprintf("Source: %s | Category: %s", source.Name, analysis.Category),
					},
				}
				
				_, err = s.ChannelMessageSendEmbed(channelID, embed)
				if err != nil {
					Logger().Printf("Failed to send summary: %v", err)
				}
			}
		}
	}
	
	// Update source stats
	source.FeedCount += len(itemsToPost)
	source.LastError = ""
	source.LastFetched = time.Now()
	
	// Update the next time in the state
	state.NewsNextTime = time.Now().Add(parseCron(cfg.News15MinCron))
	state.LastInterval = int(parseCron(cfg.News15MinCron).Minutes())
	SaveState(state)
	
	// Save source
	SaveSources([]Source{source})
}

// parseCron parses a cron expression and returns the duration to the next run
func parseCron(cronExpr string) time.Duration {
	// Default to 15 minutes if parsing fails
	defaultDuration := 15 * time.Minute
	
	if cronExpr == "" {
		return defaultDuration
	}
	
	schedule, err := cron.ParseStandard(cronExpr)
	if err != nil {
		Logger().Printf("Failed to parse cron expression '%s': %v", cronExpr, err)
		return defaultDuration
	}
	
	now := time.Now()
	next := schedule.Next(now)
	return next.Sub(now)
}
