package main

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
)

// handleInteraction processes Discord slash commands
func handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	switch i.ApplicationCommandData().Name {
	case "ping":
		handlePingCommand(s, i)
	case "status":
		handleStatusCommand(s, i)
	case "version":
		handleVersionCommand(s, i)
	}
}

// handlePingCommand responds to the ping command
func handlePingCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Pong! Bot is online and responsive.",
		},
	})
}

// handleStatusCommand shows the current status of the bot
func handleStatusCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	sources, err := LoadSources()
	if err != nil {
		respondWithError(s, i, "Failed to load sources")
		return
	}

	state, err := LoadState()
	if err != nil {
		respondWithError(s, i, "Failed to load state")
		return
	}

	// Build status message
	content := fmt.Sprintf("**Bot Status**\n")
	content += fmt.Sprintf("- Version: %s\n", cfg.Version)
	content += fmt.Sprintf("- Running since: %s\n", state.StartupTime.Format(time.RFC1123))
	content += fmt.Sprintf("- Sources: %d\n", len(sources))
	content += fmt.Sprintf("- Feed errors: %d\n", state.ErrorCount)
	content += fmt.Sprintf("- Next scheduled update: %s\n", state.NewsNextTime.Format(time.RFC1123))

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
		},
	})
}

// handleVersionCommand shows version information
func handleVersionCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	content := fmt.Sprintf("Sankarea News Bot v%s\n", cfg.Version)
	content += "A Discord bot for fetching and posting RSS feed updates."

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
		},
	})
}

// respondWithError sends an error response
func respondWithError(s *discordgo.Session, i *discordgo.InteractionCreate, errorMsg string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "⚠️ Error: " + errorMsg,
		},
	})
}
