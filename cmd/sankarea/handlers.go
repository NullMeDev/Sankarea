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
	case "filter":
		handleFilterCommand(s, i)
	case "digest":
		handleDigestCommand(s, i)
	case "track":
		handleTrackCommand(s, i)
	case "language":
		handleLanguageCommand(s, i)
	default:
		respondWithError(s, i, "Unknown command")
	}
}

// CheckCommandPermissions checks if the user has permission to use the command
func CheckCommandPermissions(s *discordgo.Session, i *discordgo.InteractionCreate) bool {
	// Allow all basic commands
	switch i.ApplicationCommandData().Name {
	case "ping", "status", "version", "help":
		return true
	}

	// For admin commands, check if user is admin
	if i.ApplicationCommandData().Name == "admin" {
		return IsAdmin(i.Member.User.ID)
	}

	// Allow all other commands by default
	return true
}

// getOptionString gets a string option from the options array
func getOptionString(options []*discordgo.ApplicationCommandInteractionDataOption, name string) string {
	for _, opt := range options {
		if opt.Name == name {
			switch opt.Type {
			case discordgo.ApplicationCommandOptionString:
				return opt.StringValue()
			}
		}
	}
	return ""
}

// getOptionInt gets an integer option from the options array
func getOptionInt(options []*discordgo.ApplicationCommandInteractionDataOption, name string) int64 {
	for _, opt := range options {
		if opt.Name == name {
			switch opt.Type {
			case discordgo.ApplicationCommandOptionInteger:
				return opt.IntValue()
			}
		}
	}
	return 0
}

// getOptionBool gets a boolean option from the options array
func getOptionBool(options []*discordgo.ApplicationCommandInteractionDataOption, name string) bool {
	for _, opt := range options {
		if opt.Name == name {
			switch opt.Type {
			case discordgo.ApplicationCommandOptionBoolean:
				return opt.BoolValue()
			}
		}
	}
	return false
}

// hasOption checks if an option exists in the options array
func hasOption(options []*discordgo.ApplicationCommandInteractionDataOption, name string) bool {
	for _, opt := range options {
		if opt.Name == name {
			return true
		}
	}
	return false
}

// respondWithError responds to an interaction with an error message
func respondWithError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "⚠️ " + message,
		},
	})
}

// followupWithError sends a followup message with an error
func followupWithError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: "⚠️ " + message,
	})
}

// stringPtr returns a pointer to a string
func stringPtr(s string) *string {
	return &s
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
				FormatDuration(uptime)),
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
	statusMessage.WriteString(fmt.Sprintf("📊 **
