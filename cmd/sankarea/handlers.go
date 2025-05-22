package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// handleInteraction processes Discord slash commands
func handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	// Check permissions
	if !CheckCommandPermissions(s, i) {
		respondWithError(s, i, "You don't have permission to use this command")
		return
	}

	// Handle commands
	switch i.ApplicationCommandData().Name {
	case "ping":
		handlePingCommand(s, i)
	case "status":
		handleStatusCommand(s, i)
	case "version":
		handleVersionCommand(s, i)
	case "source":
		handleSourceCommand(s, i)
	case "admin":
		handleAdminCommand(s, i)
	case "factcheck":
		handleFactCheckCommand(s, i)
	case "summarize":
		handleSummarizeCommand(s, i)
	case "help":
		handleHelpCommand(s, i)
	default:
		respondWithError(s, i, "Unknown command")
	}
}

// handlePingCommand responds to the ping command
func handlePingCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	startTime := time.Now()
	
	// Calculate uptime
	state, err := LoadState()
	if err != nil {
		respondWithError(s, i, "Failed to load state")
		return
	}

	uptime := time.Since(state.StartupTime).Round(time.Second)

	// Respond with latency
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("🏓 Pong! Latency: %dms | Uptime: %s", 
				time.Since(startTime).Milliseconds(),
				uptime),
		},
	})
}

// handleStatusCommand shows the current status of the bot
func handleStatusCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Acknowledge the interaction first (gives us time to gather data)
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	// Load data
	sources, err := LoadSources()
	if err != nil {
		followupWithError(s, i, "Failed to load sources")
		return
	}

	state, err := LoadState()
	if err != nil {
		followupWithError(s, i, "Failed to load state")
		return
	}

	// Count active sources and articles
	activeSources := 0
	totalArticles := state.TotalArticles
	for _, src := range sources {
		if !src.Paused {
			activeSources++
		}
	}

	// Build status message
	var statusMessage strings.Builder
	statusMessage.WriteString("**Sankarea Bot Status**\n\n")
	statusMessage.WriteString(fmt.Sprintf("📊 **General**\n"))
	statusMessage.WriteString(fmt.Sprintf("• Version: %s\n", cfg.Version))
	statusMessage.WriteString(fmt.Sprintf("• Uptime: %s\n", time.Since(state.StartupTime).Round(time.Second)))
	statusMessage.WriteString(fmt.Sprintf("• Status: %s\n", getStatusEmoji(state)))
	
	statusMessage.WriteString(fmt.Sprintf("\n📰 **News**\n"))
	statusMessage.WriteString(fmt.Sprintf("• Active Sources: %d/%d\n", activeSources, len(sources)))
	statusMessage.WriteString(fmt.Sprintf("• Articles Processed: %d\n", totalArticles))
	statusMessage.WriteString(fmt.Sprintf("• Update Interval: %d minutes\n", cfg.NewsIntervalMinutes))
	statusMessage.WriteString(fmt.Sprintf("• Next Update: <t:%d:R>\n", state.NewsNextTime.Unix()))
	statusMessage.WriteString(fmt.Sprintf("• Last Digest: %s\n", formatTimeOrNever(state.LastDigest)))
	
	statusMessage.WriteString(fmt.Sprintf("\n🔧 **System**\n"))
	statusMessage.WriteString(fmt.Sprintf("• Errors: %d\n", state.ErrorCount))
	
	// Feature status
	statusMessage.WriteString(fmt.Sprintf("\n🛠️ **Features**\n"))
	statusMessage.WriteString(fmt.Sprintf("• Fact Checking: %s\n", getEnabledStatus(cfg.EnableFactCheck)))
	statusMessage.WriteString(fmt.Sprintf("• Summarization: %s\n", getEnabledStatus(cfg.EnableSummarization)))
	statusMessage.WriteString(fmt.Sprintf("• Content Filtering: %s\n", getEnabledStatus(cfg.EnableContentFiltering)))

	// Update with full status
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &statusMessage.String(),
	})
}

// getStatusEmoji returns an emoji representing the bot status
func getStatusEmoji(state *State) string {
	if state.Paused {
		return "⏸️ Paused"
	}
	if state.ErrorCount > 0 {
		return "⚠️ Warning"
	}
	return "✅ Running"
}

// getEnabledStatus returns a string indicating if a feature is enabled
func getEnabledStatus(enabled bool) string {
	if enabled {
		return "✅ Enabled"
	}
	return "❌ Disabled"
}

// formatTimeOrNever formats a time or returns "Never" if it's zero
func formatTimeOrNever(t time.Time) string {
	if t.IsZero() {
		return "Never"
	}
	return fmt.Sprintf("<t:%d:R>", t.Unix())
}

// handleVersionCommand shows version information
func handleVersionCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{
				{
					Title:       "Sankarea News Bot",
					Description: "A Discord bot for fetching and posting RSS feed updates with fact checking and summarization.",
					Color:       0x4B9CD3, // Blue color
					Fields: []*discordgo.MessageEmbedField{
						{
							Name:   "Version",
							Value:  cfg.Version,
							Inline: true,
						},
						{
							Name:   "Author",
							Value:  "[NullMeDev](https://github.com/NullMeDev)",
							Inline: true,
						},
						{
							Name:   "Repository",
							Value:  "[GitHub](https://github.com/NullMeDev/sankarea)",
							Inline: true,
						},
					},
					Footer: &discordgo.MessageEmbedFooter{
						Text: "Type /help for available commands",
					},
					Timestamp: time.Now().Format(time.RFC3339),
				},
			},
		},
	})
}

// handleSourceCommand manages RSS feed sources
func handleSourceCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	subCommand := options[0].Name

	switch subCommand {
	case "add":
		handleSourceAdd(s, i, options[0])
	case "remove":
		handleSourceRemove(s, i, options[0])
	case "list":
		handleSourceList(s, i)
	case "update":
		handleSourceUpdate(s, i, options[0])
	default:
		respondWithError(s, i, "Unknown subcommand")
	}
}

// handleSourceAdd adds a new RSS feed source
func handleSourceAdd(s *discordgo.Session, i *discordgo.InteractionCreate, option *discordgo.ApplicationCommandInteractionDataOption) {
	// Acknowledge the interaction first
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	options := option.Options
	name := getOptionString(options, "name")
	url := getOptionString(options, "url")
	category := getOptionString(options, "category")
	factCheck := getOptionBool(options, "fact_check")
	summarize := getOptionBool(options, "summarize")
	channelOverride := getOptionString(options, "channel")

	// Load existing sources
	sources, err := LoadSources()
	if err != nil {
		followupWithError(s, i, "Failed to load sources")
		return
	}

	// Check if source already exists
	for _, src := range sources {
		if strings.EqualFold(src.Name, name) {
			followupWithError(s, i, "A source with that name already exists")
			return
		}
	}

	// Validate URL format
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		followupWithError(s, i, "Invalid URL format. URL must start with http:// or https://")
		return
	}

	// Try to parse the feed to validate
	parser := newFeedParser()
	_, err = parser.ParseURL(url)
	if err != nil {
		followupWithError(s, i, fmt.Sprintf("Invalid RSS feed: %v", err))
		return
	}

	// Create new source
	newSource := Source{
		Name:            name,
		URL:             url,
		Paused:          false,
		LastDigest:      time.Time{},
		LastFetched:     time.Time{},
		NewsNextTime:    time.Now(),
		Category:        category,
		FactCheckAuto:   factCheck && cfg.EnableFactCheck,
		SummarizeAuto:   summarize && cfg.EnableSummarization,
		TrustScore:      0.5, // Default neutral score
		ChannelOverride: channelOverride,
	}

	// Add to sources
	sources = append(sources, newSource)
	if err := SaveSources(sources); err != nil {
		followupWithError(s, i, "Failed to save sources")
		return
	}

	// Log action to audit log
	AuditLog(s, "Source Add", i.Member.User.ID, fmt.Sprintf("Added source '%s' (%s)", name, url))

	// Send success message with embed
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &[]*discordgo.MessageEmbed{
			{
				Title:       "RSS Feed Added",
				Description: "New RSS feed source has been added successfully.",
				Color:       0x43B581, // Green
				Fields: []*discordgo.MessageEmbedField{
					{
						Name:   "Name",
						Value:  name,
						Inline: true,
					},
					{
						Name:   "URL",
						Value:  url,
						Inline: true,
					},
					{
						Name:   "Category",
						Value:  ifEmpty(category, "None"),
						Inline: true,
					},
					{
						Name:   "Fact Checking",
						Value:  getEnabledStatus(factCheck && cfg.EnableFactCheck),
						Inline: true,
					},
					{
						Name:   "Summarization",
						Value:  getEnabledStatus(summarize && cfg.EnableSummarization),
						Inline: true,
					},
					{
						Name:   "Channel Override",
						Value:  ifEmpty(channelOverride, "Default"),
						Inline: true,
					},
				},
				Timestamp: time.Now().Format(time.RFC3339),
			},
		},
	})
}

// handleSourceRemove removes an RSS feed source
func handleSourceRemove(s *discordgo.Session, i *discordgo.InteractionCreate, option *discordgo.ApplicationCommandInteractionDataOption) {
	// Acknowledge the interaction first
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	name := getOptionString(option.Options, "name")

	// Load existing sources
	sources, err := LoadSources()
	if err != nil {
		followupWithError(s, i, "Failed to load sources")
		return
	}

	// Find source index
	var found bool
	var index int
	for i, src := range sources {
		if strings.EqualFold(src.Name, name) {
			found = true
			index = i
			break
		}
	}

	if !found {
		followupWithError(s, i, "Source not found")
		return
	}

	// Remove source (preserve order by copying the last element to the removed position)
	sources[index] = sources[len(sources)-1]
	sources = sources[:len(sources)-1]

	// Save sources
	if err := SaveSources(sources); err != nil {
		followupWithError(s, i, "Failed to save sources")
		return
	}

	// Log action to audit log
	AuditLog(s, "Source Remove", i.Member.User.ID, fmt.Sprintf("Removed source '%s'", name))

	// Send success message
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: stringPtr(fmt.Sprintf("🗑️ Successfully removed source: **%s**", name)),
	})
}

// handleSourceList lists all RSS feed sources
func handleSourceList(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Acknowledge the interaction first
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	// Load sources
	sources, err := LoadSources()
	if err != nil {
		followupWithError(s, i, "Failed to load sources")
		return
	}

	if len(sources) == 0 {
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: stringPtr("No sources have been added yet. Use `/source add` to add an RSS feed."),
		})
		return
	}

	// Group sources by category
	categories := make(map[string][]Source)
	for _, src := range sources {
		cat := "Uncategorized"
		if src.Category != "" {
			cat = src.Category
		}
		categories[cat] = append(categories[cat], src)
	}

	// Create embed fields for each category
	var categoryNames []string
	for category := range categories {
		categoryNames = append(categoryNames, category)
	}
	sortStrings(categoryNames) // Sort categories alphabetically

	// Build embeds (Discord has a limit of 25 fields per embed)
	var embeds []*discordgo.MessageEmbed
	currentEmbed := &discordgo.MessageEmbed{
		Title:       "RSS Feed Sources",
		Description: fmt.Sprintf("Total: %d sources", len(sources)),
		Color:       0x4B9CD3, // Blue
		Fields:      []*discordgo.MessageEmbedField{},
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	fieldCount := 0
	maxFieldsPerEmbed := 25

	for _, category := range categoryNames {
		srcList := categories[category]

		// Create source list string
		var sourcesList strings.Builder
		for _, src := range srcList {
			statusEmoji := "🟢" // Active
			if src.Paused {
				statusEmoji = "🔴" // Paused
			}
			sourcesList.WriteString(fmt.Sprintf("%s **%s** - %s\n", statusEmoji, src.Name, truncateString(src.URL, 50)))
		}

		// Add field to current embed
		currentEmbed.Fields = append(currentEmbed.Fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("📁 %s (%d)", category, len(srcList)),
			Value:  sourcesList.String(),
			Inline: false,
		})

		fieldCount++

		// If we've reached the limit, start a new embed
		if fieldCount >= maxFieldsPerEmbed {
			embeds = append(embeds, currentEmbed)
			currentEmbed = &discordgo.MessageEmbed{
				Title:     "RSS Feed Sources (Continued)",
				Color:     0x4B9CD3, // Blue
				Fields:    []*discordgo.MessageEmbedField{},
				Timestamp: time.Now().Format(time.RFC3339),
			}
			fieldCount = 0
		}
	}

	// Add the last embed if it's not empty
	if len(currentEmbed.Fields) > 0 {
		embeds = append(embeds, currentEmbed)
	}

	// Can only send up to 10 embeds at once
	if len(embeds) > 10 {
		embeds = embeds[:10]
		// Append note about truncation
		embeds[9].Footer = &discordgo.MessageEmbedFooter{
			Text: "Source list truncated due to Discord limits. Use more specific category filtering.",
		}
	}

	// Send the message
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &embeds,
	})
}

// handleSourceUpdate updates an existing RSS feed source
func handleSourceUpdate(s *discordgo.Session, i *discordgo.InteractionCreate, option *discordgo.ApplicationCommandInteractionDataOption) {
	// Acknowledge the interaction first
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	options := option.Options
	name := getOptionString(options, "name")

	// Load existing sources
	sources, err := LoadSources()
	if err != nil {
		followupWithError(s, i, "Failed to load sources")
		return
	}

	// Find source
	var found bool
	var index int
	for i, src := range sources {
		if strings.EqualFold(src.Name, name) {
			found = true
			index = i
			break
		}
	}

	if !found {
		followupWithError(s, i, "Source not found")
		return
	}

	// Update fields if provided
	changes := []string{}

	// URL
	if url := getOptionString(options, "url"); url != "" {
		// Validate URL
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			followupWithError(s, i, "Invalid URL format. URL must start with http:// or https://")
			return
		}

		// Try to parse the feed
		parser := newFeedParser()
		_, err := parser.ParseURL(url)
		if err != nil {
			followupWithError(s, i, fmt.Sprintf("Invalid RSS feed: %v", err))
			return
		}

		oldURL := sources[index].URL
		sources[index].URL = url
		changes = append(changes, fmt.Sprintf("URL: %s → %s", truncateString(oldURL, 30), truncateString(url, 30)))
	}

	// Category
	if hasOption(options, "category") {
		oldCategory := sources[index].Category
		newCategory := getOptionString(options, "category")
		sources[index].Category = newCategory
		changes = append(changes, fmt.Sprintf("Category: %s → %s", ifEmpty(oldCategory, "None"), ifEmpty(newCategory, "None")))
	}

	// Fact check
	if hasOption(options, "fact_check") {
		oldFactCheck := sources[index].FactCheckAuto
		newFactCheck := getOptionBool(options, "fact_check") && cfg.EnableFactCheck
		sources[index].FactCheckAuto = newFactCheck
		changes = append(changes, fmt.Sprintf("Fact Check: %s → %s", getEnabledStatus(oldFactCheck), getEnabledStatus(newFactCheck)))
	}

	// Summarize
	if hasOption(options, "summarize") {
		oldSummarize := sources[index].SummarizeAuto
		newSummarize := getOptionBool(options, "summarize") && cfg.EnableSummarization
		sources[index].SummarizeAuto = newSummarize
		changes = append(changes, fmt.Sprintf("Summarize: %s → %s", getEnabledStatus(oldSummarize), getEnabledStatus(newSummarize)))
	}

	// Channel override
	if hasOption(options, "channel") {
		oldChannel := sources[index].ChannelOverride
		newChannel := getOptionString(options, "channel")
		sources[index].ChannelOverride = newChannel
		changes = append(changes, fmt.Sprintf("Channel: %s → %s", ifEmpty(oldChannel, "Default"), ifEmpty(newChannel, "Default")))
	}

	// Pause status
	if hasOption(options, "pause") {
		oldPause := sources[index].Paused
		newPause := getOptionBool(options, "pause")
		sources[index].Paused = newPause
		changes = append(changes, fmt.Sprintf("Status: %s → %s", 
			getStatusText(oldPause), 
			getStatusText(newPause)))
	}

	if len(changes) == 0 {
		followupWithError(s, i, "No changes were specified")
		return
	}

	// Save sources
	if err := SaveSources(sources); err != nil {
		followupWithError(s, i, "Failed to save sources")
		return
	}

	// Log action to audit log
	AuditLog(s, "Source Update", i.Member.User.ID, fmt.Sprintf("Updated source '%s'", name))

	// Create changelog
	var changeLog strings.Builder
	for _, change := range changes {
		changeLog.WriteString("• " + change + "\n")
	}

	// Send success message with embed
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &[]*discordgo.MessageEmbed{
			{
				Title:       "RSS Feed Updated",
				Description: fmt.Sprintf("Source **%s** has been updated.", name),
				Color:       0x4B9CD3, // Blue
				Fields: []*discordgo.MessageEmbedField{
					{
						Name:   "Changes",
						Value:  changeLog.String(),
						Inline: false,
					},
				},
				Timestamp: time.Now().Format(time.RFC3339),
			},
		},
	})
}

// getStatusText returns text representation of pause status
func getStatusText(paused bool) string {
	if paused {
		return "🔴 Paused"
	}
	return "🟢 Active"
}

// handleAdminCommand handles administrative commands
func handleAdminCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	subCommand := options[0].Name

	switch subCommand {
	case "pause":
		handleAdminPause(s, i)
	case "resume":
		handleAdminResume(s, i)
	case "interval":
		handleAdminInterval(s, i, options[0])
	case "refresh":
		handleAdminRefresh(s, i)
	case "stats":
		handleAdminStats(s, i)
	case "reset":
		handleAdminReset(s, i)
	case "config":
		handleAdminConfig(s, i, options[0])
	default:
		respondWithError(s, i, "Unknown subcommand")
	}
}

// handleAdminPause pauses news updates
func handleAdminPause(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Acknowledge the interaction first
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	// Load state
	state, err := LoadState()
	if err != nil {
		followupWithError(s, i, "Failed to load state")
		return
	}

	// Check if already paused
	if state.Paused {
		followupWithError(s, i, "News updates are already paused")
		return
	}

	// Update state
	state.Paused = true
	state.LockdownSetBy = i.Member.User.Username
	if err := SaveState(state); err != nil {
		followupWithError(s, i, "Failed to save state")
		return
	}

	// Log action to audit log
	AuditLog(s, "System Pause", i.Member.User.ID, "Paused all news updates")

	// Send success message
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: stringPtr("⏸️ News updates have been **paused**. Use `/admin resume` to resume updates."),
	})
}

// handleAdminResume resumes news updates
func handleAdminResume(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Acknowledge the interaction first
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	// Load state
	state, err := LoadState()
	if err != nil {
		followupWithError(s, i, "Failed to load state")
		return
	}

	// Check if already running
	if !state.Paused {
		followupWithError(s, i, "News updates are already running")
		return
	}

	// Update state
	state.Paused = false
	state.LockdownSetBy = ""
	state.NewsNextTime = time.Now().Add(time.Duration(cfg.NewsIntervalMinutes) * time.Minute)
	if err := SaveState(state); err != nil {
		followupWithError(s, i, "Failed to save state")
		return
	}

	// Log action to audit log
	AuditLog(s, "System Resume", i.Member.User.ID, "Resumed news updates")

	// Send success message
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: stringPtr(fmt.Sprintf("▶️ News updates have been **resumed**. Next update in <t:%d:R>.", state.NewsNextTime.Unix())),
	})
}

// handleAdminInterval changes the news update interval
func handleAdminInterval(s *discordgo.Session, i *discordgo.InteractionCreate, option *discordgo.ApplicationCommandInteractionDataOption) {
	// Acknowledge the interaction first
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	// Get minutes
	minutes := int(getOptionInt(option.Options, "minutes"))
	if minutes < 5 {
		followupWithError(s, i, "Interval must be at least 5 minutes")
		return
	}

	// Load config and update
	config := *cfg // Make a copy
	oldInterval := config.NewsIntervalMinutes
	config.NewsIntervalMinutes = minutes
	
	// Save config
	if err := SaveConfig(&config); err != nil {
		followupWithError(s, i, "Failed to save config")
		return
	}

	// Reload config
	var err error
	cfg, err = LoadConfig()
	if err != nil {
		followupWithError(s, i, "Failed to reload config")
		return
	}

	// Update cron job
	var entryID cron.EntryID
	UpdateCronJob(cronManager, &entryID, minutes, s, cfg.NewsChannelID, nil)

	// Update state for next update time
	state, err := LoadState()
	if err == nil {
		state.NewsNextTime = time.Now().Add(time.Duration(minutes) * time.Minute)
		state.LastInterval = minutes
		SaveState(state)
	}

	// Log action to audit log
	AuditLog(s, "Interval Change", i.Member.User.ID, fmt.Sprintf("Changed news interval from %d to %d minutes", oldInterval, minutes))

	// Send success message
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: stringPtr(fmt.Sprintf("⏱️ News update interval changed from **%d** to **%d** minutes. Next update in <t:%d:R>.", 
			oldInterval, minutes, time.Now().Add(time.Duration(minutes)*time.Minute).Unix())),
	})
}

// handleAdminRefresh forces a refresh of all news sources
func handleAdminRefresh(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Acknowledge the interaction first
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	// Load sources
	sources, err := LoadSources()
	if err != nil {
		followupWithError(s, i, "Failed to load sources")
		return
	}

	// Check if we have any sources
	if len(sources) == 0 {
		followupWithError(s, i, "No sources configured")
		return
	}

	// Log action to audit log
	AuditLog(s, "Refresh", i.Member.User.ID, "Forced refresh of all news sources")

	// Send initial response
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: stringPtr(fmt.Sprintf("🔄 Refreshing %d news sources...", len(sources))),
	})

	// Start the refresh in a goroutine
	go func() {
		// Force refresh all sources
		forceFetchAllSources(s, sources)

		// Update the message after refresh is done
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: stringPtr(fmt.Sprintf("✅ Refresh complete! Processed %d news sources.", len(sources))),
		})
	}()
}

// handleAdminStats shows detailed bot statistics
func handleAdminStats(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Acknowledge the interaction first
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	// Load state
	state, err := LoadState()
	if err != nil {
		followupWithError(s, i, "Failed to load state")
		return
	}

	// Load sources
	sources, err := LoadSources()
	if err != nil {
		followupWithError(s, i, "Failed to load sources")
		return
	}

	// Count active sources and sources with errors
	activeSources := 0
	sourcesWithErrors := 0
	totalPosts := 0
	for _, src := range sources {
		if !src.Paused {
			activeSources++
		}
		if src.LastError != "" {
			sourcesWithErrors++
		}
		totalPosts += src.FeedCount
	}

	// Build statistics message
	var stats strings.Builder
	stats.WriteString("**📊 Sankarea Bot Statistics**\n\n")
	
	// Source statistics
	stats.WriteString("**Sources**\n")
	stats.WriteString(fmt.Sprintf("• Total Sources: %d\n", len(sources)))
	stats.WriteString(fmt.Sprintf("• Active Sources: %d\n", activeSources))
	stats.WriteString(fmt.Sprintf("• Paused Sources: %d\n", len(sources)-activeSources))
	stats.WriteString(fmt.Sprintf("• Sources with Errors: %d\n", sourcesWithErrors))
	
	// Post statistics
	stats.WriteString("\n**Activity**\n")
	stats.WriteString(fmt.Sprintf("• Total Articles Processed: %d\n", state.TotalArticles))
	stats.WriteString(fmt.Sprintf("• Total Posts Sent: %d\n", totalPosts))
	stats.WriteString(fmt.Sprintf("• Last Digest: %s\n", formatTimeOrNever(state.LastDigest)))
	stats.WriteString(fmt.Sprintf("• Bot Uptime: %s\n", time.Since(state.StartupTime).Round(time.Second)))
	
	// API usage
	if cfg.EnableSummarization || cfg.EnableFactCheck {
		stats.WriteString("\n**API Usage**\n")
		if cfg.EnableSummarization {
			stats.WriteString(fmt.Sprintf("• OpenAI API (Daily): $%.4f\n", state.DailyAPIUsage))
			stats.WriteString(fmt.Sprintf("• Last Summary Cost: $%.4f\n", state.LastSummaryCost))
		}
		if cfg.EnableFactCheck {
			stats.WriteString(fmt.Sprintf("• Fact Check Requests: %d\n", state.ErrorCount)) // Using ErrorCount temporarily
		}
	}
	
	// Error statistics
	stats.WriteString("\n**System Health**\n")
	stats.WriteString(fmt.Sprintf("• Total Errors: %d\n", state.ErrorCount))
	stats.WriteString(fmt.Sprintf("• System Status: %s\n", getStatusEmoji(state)))

	// Source table (top 5 most active)
	stats.WriteString("\n**Top Sources by Activity**\n")
	
	// Sort sources by feed count
	sortedSources := make([]Source, len(sources))
	copy(sortedSources, sources)
	sortSourcesByActivity(sortedSources)
	
	// Take top 5
	limit := 5
	if len(sortedSources) < limit {
		limit = len(sortedSources)
	}
	
	// Add source table
	for i := 0; i < limit; i++ {
		src := sortedSources[i]
		statusEmoji := "🟢"
		if src.Paused {
			statusEmoji = "🔴"
		} else if src.LastError != "" {
			statusEmoji = "⚠️"
		}
		stats.WriteString(fmt.Sprintf("%s **%s**: %d posts\n", statusEmoji, src.Name, src.FeedCount))
	}

	// Send statistics
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: stringPtr(stats.String()),
	})
}

// handleAdminReset resets error counts and clears error states
func handleAdminReset(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Acknowledge the interaction first
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	// Load state and sources
	state, err := LoadState()
	if err != nil {
		followupWithError(s, i, "Failed to load state")
		return
	}
	
	sources, err := LoadSources()
	if err != nil {
		followupWithError(s, i, "Failed to load sources")
		return
	}

	// Reset state errors
	oldErrorCount := state.ErrorCount
	state.ErrorCount = 0
	state.LastError = ""
	
	if err := SaveState(state); err != nil {
		followupWithError(s, i, "Failed to save state")
		return
	}

	// Reset source errors
	sourceErrorsCount := 0
	for i := range sources {
		if sources[i].LastError != "" {
			sourceErrorsCount++
			sources[i].LastError = ""
			sources[i].ErrorCount = 0
		}
	}
	
	if sourceErrorsCount > 0 {
		if err := SaveSources(sources); err != nil {
			followupWithError(s, i, "Failed to save sources")
			return
		}
	}

	// Log action to audit log
	AuditLog(s, "Error Reset", i.Member.User.ID, fmt.Sprintf("Reset %d system errors and %d source errors", oldErrorCount, sourceErrorsCount))

	// Send success message
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: stringPtr(fmt.Sprintf("✅ Successfully reset %d system errors and %d source errors.", oldErrorCount, sourceErrorsCount)),
	})
}

// handleAdminConfig updates bot configuration
func handleAdminConfig(s *discordgo.Session, i *discordgo.InteractionCreate, option *discordgo.ApplicationCommandInteractionDataOption) {
	// Acknowledge the interaction first
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	options := option.Options
	if len(options) == 0 {
		followupWithError(s, i, "No configuration changes specified")
		return
	}

	// Load config
	config := *cfg // Make a copy
	changes := []string{}

	// Update digest schedule
	if digestSchedule := getOptionString(options, "digest_schedule"); digestSchedule != "" {
		// Validate cron expression
		if !validateCronExpression(digestSchedule) {
			followupWithError(s, i, "Invalid cron expression for digest schedule")
			return
		}
		
		oldSchedule := config.DigestCronSchedule
		config.DigestCronSchedule = digestSchedule
		changes = append(changes, fmt.Sprintf("Digest Schedule: %s → %s", oldSchedule, digestSchedule))
	}

	// Update max posts
	if hasOption(options, "max_posts") {
		maxPosts := int(getOptionInt(options, "max_posts"))
		oldMaxPosts := config.MaxPostsPerSource
		config.MaxPostsPerSource = maxPosts
		changes = append(changes, fmt.Sprintf("Max Posts: %d → %d", oldMaxPosts, maxPosts))
	}

	// Update fact check toggle
	if hasOption(options, "enable_fact_check") {
		oldFactCheck := config.EnableFactCheck
		newFactCheck := getOptionBool(options, "enable_fact_check")
		
		// Check if we have API keys configured when enabling
		if newFactCheck && (config.GoogleFactCheckAPIKey == "" && config.ClaimBustersAPIKey == "") {
			followup
