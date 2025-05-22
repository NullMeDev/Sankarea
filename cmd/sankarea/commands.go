package main

import (
	"github.com/bwmarrin/discordgo"
)

// Command permission levels
const (
	PermLevelEveryone = 0
	PermLevelAdmin    = 1
	PermLevelOwner    = 2
)

// RegisterCommands creates all slash commands
func RegisterCommands(s *discordgo.Session, appID, guildID string) error {
	cmds := []*discordgo.ApplicationCommand{
		{
			Name:        "ping",
			Description: "Check if the bot is alive",
		},
		{
			Name:        "status",
			Description: "Show current status of the bot",
		},
		{
			Name:        "version",
			Description: "Show bot version information",
		},

		// Source Management Commands
		{
			Name:        "source",
			Description: "Manage news sources",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "add",
					Description: "Add a new news source",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "name",
							Description: "Name of the news source",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "url",
							Description: "RSS feed URL",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "category",
							Description: "Category for the news source",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionBoolean,
							Name:        "fact_check",
							Description: "Enable automatic fact checking for this source",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionBoolean,
							Name:        "summarize",
							Description: "Enable automatic summarization for this source",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "channel",
							Description: "Override default channel for this source (channel ID)",
							Required:    false,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "remove",
					Description: "Remove an existing news source",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "name",
							Description: "Name of the news source to remove",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "list",
					Description: "List all news sources",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "update",
					Description: "Update an existing news source",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "name",
							Description: "Name of the news source to update",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "url",
							Description: "New RSS feed URL",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "category",
							Description: "Category for the news source",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionBoolean,
							Name:        "fact_check",
							Description: "Enable automatic fact checking for this source",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionBoolean,
							Name:        "summarize",
							Description: "Enable automatic summarization for this source",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "channel",
							Description: "Override default channel for this source (channel ID)",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionBoolean,
							Name:        "pause",
							Description: "Pause this source",
							Required:    false,
						},
					},
				},
			},
		},

		// Admin Commands
		{
			Name:        "admin",
			Description: "Administrative commands for the bot",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "pause",
					Description: "Pause all news updates",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "resume",
					Description: "Resume news updates",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "interval",
					Description: "Change news update interval",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionInteger,
							Name:        "minutes",
							Description: "News update interval in minutes",
							Required:    true,
							MinValue:    &[]float64{5}[0],  // Minimum 5 minutes
							MaxValue:    1440, // Maximum 24 hours
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "refresh",
					Description: "Force refresh news from all sources",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "stats",
					Description: "Show detailed bot statistics",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "reset",
					Description: "Reset error counts and clear error states",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "config",
					Description: "Update bot configuration",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "digest_schedule",
							Description: "Cron schedule for daily digest (e.g. '0 8 * * *')",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionInteger,
							Name:        "max_posts",
							Description: "Maximum posts per source",
							Required:    false,
							MinValue:    &[]float64{1}[0],
							MaxValue:    20,
						},
						{
							Type:        discordgo.ApplicationCommandOptionBoolean,
							Name:        "enable_fact_check",
							Description: "Enable fact checking globally",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionBoolean,
							Name:        "enable_summarization",
							Description: "Enable summarization globally",
							Required:    false,
						},
					},
				},
			},
		},

		// Fact-checking Commands
		{
			Name:        "factcheck",
			Description: "Fact check an article or claim",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "url",
					Description: "URL of the article to fact check",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "claim",
					Description: "Claim text to fact check",
					Required:    false,
				},
			},
		},

		// Summarization Commands
		{
			Name:        "summarize",
			Description: "Summarize an article",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "url",
					Description: "URL of the article to summarize",
					Required:    true,
				},
			},
		},

		// Help Command
		{
			Name:        "help",
			Description: "Show help information",
		},
	}

	// Register each command
	for _, cmd := range cmds {
		_, err := s.ApplicationCommandCreate(appID, guildID, cmd)
		if err != nil {
			return err
		}
	}

	return nil
}

// CommandRequiresAdmin returns true if the command requires admin permissions
func CommandRequiresAdmin(commandName string) bool {
	switch commandName {
	case "admin":
		return true
	case "source":
		return true
	default:
		return false
	}
}

// CommandRequiresOwner returns true if the command requires owner permissions
func CommandRequiresOwner(commandName string) bool {
	// Currently only subcommands of admin require owner permissions
	// This is checked in handlers.go
	return false
}
