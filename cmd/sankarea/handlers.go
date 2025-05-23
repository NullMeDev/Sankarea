package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

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

// handleSourceCommand handles the /source command
func handleSourceCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Get subcommand
	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		respondWithError(s, i, "Missing subcommand")
		return
	}
	
	subcommand := options[0].Name
	
	switch subcommand {
	case "list":
		handleSourceList(s, i)
	case "add":
		handleSourceAdd(s, i, options[0])
	case "remove":
		handleSourceRemove(s, i, options[0])
	case "info":
		handleSourceInfo(s, i, options[0])
	default:
		respondWithError(s, i, "Unknown subcommand")
	}
}

// handleSourceList shows a list of all sources
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
			Content: stringPtr("No news sources configured. Use `/source add` to add some."),
		})
		return
	}

	// Group sources by category
	categories := make(map[string][]Source)
	for _, source := range sources {
		cat := source.Category
		if cat == "" {
			cat = "Uncategorized"
		}
		categories[cat] = append(categories[cat], source)
	}

	// Build embed
	embed := &discordgo.MessageEmbed{
		Title:       "News Sources",
		Description: fmt.Sprintf("There are %d sources configured.", len(sources)),
		Color:       0x3498DB, // Blue
		Fields:      []*discordgo.MessageEmbedField{},
	}

	// Add categories as fields
	for category, categorySources := range categories {
		var sourceList strings.Builder
		for _, source := range categorySources {
			status := "✅"
			if source.Paused {
				status = "⏸️"
			}
			sourceList.WriteString(fmt.Sprintf("%s **%s**\n", status, source.Name))
		}
		
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  category,
			Value: sourceList.String(),
		})
	}

	// Update with embed
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &[]*discordgo.MessageEmbed{embed},
	})
}

// handleSourceAdd adds a new news source
func handleSourceAdd(s *discordgo.Session, i *discordgo.InteractionCreate, option *discordgo.ApplicationCommandInteractionDataOption) {
	// Get options
	name := getOptionString(option.Options, "name")
	url := getOptionString(option.Options, "url")
	category := getOptionString(option.Options, "category")
	
	// Validate
	if name == "" || url == "" {
		respondWithError(s, i, "Name and URL are required")
		return
	}
	
	// Acknowledge the interaction
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	
	// Check if source already exists
	sources, err := LoadSources()
	if err != nil {
		followupWithError(s, i, "Failed to load sources")
		return
	}
	
	for _, source := range sources {
		if source.Name == name {
			followupWithError(s, i, fmt.Sprintf("
