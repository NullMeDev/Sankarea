// cmd/sankarea/commands.go
package main

import (
    "fmt"
    "strings"

    "github.com/bwmarrin/discordgo"
)

// CommandHandler represents a function that handles a Discord command
type CommandHandler func(s *discordgo.Session, i *discordgo.InteractionCreate)

// commands stores the mapping of command names to their handlers
var commands = map[string]CommandHandler{
    "ping":      handlePingCommand,
    "status":    handleStatusCommand,
    "version":   handleVersionCommand,
    "source":    handleSourceCommand,
    "admin":     handleAdminCommand,
    "factcheck": handleFactCheckCommand,
    "digest":    handleDigestCommand,
    "help":      handleHelpCommand,
}

// commandDefinitions defines all slash commands and their options
var commandDefinitions = []*discordgo.ApplicationCommand{
    {
        Name:        "ping",
        Description: "Check if the bot is alive",
    },
    {
        Name:        "status",
        Description: "Show current bot status and statistics",
    },
    {
        Name:        "version",
        Description: "Show bot version information",
    },
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
                        Description: "News category",
                        Required:    true,
                        Choices: []*discordgo.ApplicationCommandOptionChoice{
                            {Name: "Technology", Value: CategoryTechnology},
                            {Name: "Business", Value: CategoryBusiness},
                            {Name: "Science", Value: CategoryScience},
                            {Name: "Health", Value: CategoryHealth},
                            {Name: "Politics", Value: CategoryPolitics},
                            {Name: "Sports", Value: CategorySports},
                            {Name: "World", Value: CategoryWorld},
                        },
                    },
                    {
                        Type:        discordgo.ApplicationCommandOptionBoolean,
                        Name:        "fact_check",
                        Description: "Enable fact-checking for this source",
                        Required:    false,
                    },
                },
            },
            {
                Type:        discordgo.ApplicationCommandOptionSubCommand,
                Name:        "remove",
                Description: "Remove a news source",
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
                Description: "Update a news source",
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
                        Description: "New RSS feed URL",
                        Required:    false,
                    },
                    {
                        Type:        discordgo.ApplicationCommandOptionString,
                        Name:        "category",
                        Description: "New category",
                        Required:    false,
                        Choices: []*discordgo.ApplicationCommandOption{
                            {Name: "Technology", Value: CategoryTechnology},
                            {Name: "Business", Value: CategoryBusiness},
                            {Name: "Science", Value: CategoryScience},
                            {Name: "Health", Value: CategoryHealth},
                            {Name: "Politics", Value: CategoryPolitics},
                            {Name: "Sports", Value: CategorySports},
                            {Name: "World", Value: CategoryWorld},
                        },
                    },
                    {
                        Type:        discordgo.ApplicationCommandOptionBoolean,
                        Name:        "paused",
                        Description: "Pause/unpause the source",
                        Required:    false,
                    },
                },
            },
        },
    },
    {
        Name:        "admin",
        Description: "Administrative commands",
        Options: []*discordgo.ApplicationCommandOption{
            {
                Type:        discordgo.ApplicationCommandOptionSubCommand,
                Name:        "pause",
                Description: "Pause news gathering",
            },
            {
                Type:        discordgo.ApplicationCommandOptionSubCommand,
                Name:        "resume",
                Description: "Resume news gathering",
            },
            {
                Type:        discordgo.ApplicationCommandOptionSubCommand,
                Name:        "reload",
                Description: "Reload configuration",
            },
            {
                Type:        discordgo.ApplicationCommandOptionSubCommand,
                Name:        "config",
                Description: "View/update config",
                Options: []*discordgo.ApplicationCommandOption{
                    {
                        Type:        discordgo.ApplicationCommandOptionInteger,
                        Name:        "max_posts",
                        Description: "Maximum posts per interval",
                        Required:    false,
                    },
                    {
                        Type:        discordgo.ApplicationCommandOptionInteger,
                        Name:        "interval",
                        Description: "News fetch interval (minutes)",
                        Required:    false,
                    },
                },
            },
        },
    },
    {
        Name:        "factcheck",
        Description: "Fact-check operations",
        Options: []*discordgo.ApplicationCommandOption{
            {
                Type:        discordgo.ApplicationCommandOptionString,
                Name:        "url",
                Description: "URL of the article to fact-check",
                Required:    true,
            },
        },
    },
    {
        Name:        "digest",
        Description: "Generate news digest",
        Options: []*discordgo.ApplicationCommandOption{
            {
                Type:        discordgo.ApplicationCommandOptionString,
                Name:        "period",
                Description: "Time period for digest",
                Required:    false,
                Choices: []*discordgo.ApplicationCommandOptionChoice{
                    {Name: "Daily", Value: "daily"},
                    {Name: "Weekly", Value: "weekly"},
                },
            },
        },
    },
    {
        Name:        "help",
        Description: "Show help information",
        Options: []*discordgo.ApplicationCommandOption{
            {
                Type:        discordgo.ApplicationCommandOptionString,
                Name:        "command",
                Description: "Specific command to get help for",
                Required:    false,
            },
        },
    },
}

// RegisterCommands registers all slash commands with Discord
func RegisterCommands(s *discordgo.Session) error {
    Logger().Println("Registering commands...")

    // Get existing commands
    existingCommands, err := s.ApplicationCommands(s.State.User.ID, "")
    if err != nil {
        return fmt.Errorf("failed to get existing commands: %v", err)
    }

    // Delete existing commands
    for _, cmd := range existingCommands {
        if err := s.ApplicationCommandDelete(s.State.User.ID, "", cmd.ID); err != nil {
            Logger().Printf("Failed to delete command %s: %v", cmd.Name, err)
        }
    }

    // Register new commands
    for _, cmd := range commandDefinitions {
        _, err := s.ApplicationCommandCreate(s.State.User.ID, "", cmd)
        if err != nil {
            return fmt.Errorf("failed to create command %s: %v", cmd.Name, err)
        }
    }

    Logger().Printf("Successfully registered %d commands", len(commandDefinitions))
    return nil
}

// HandleCommand routes command interactions to their handlers
func HandleCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
    if i.Type != discordgo.InteractionApplicationCommand {
        return
    }

    // Get command name
    cmdName := i.ApplicationCommandData().Name

    // Check if handler exists
    handler, exists := commands[cmdName]
    if !exists {
        respondWithError(s, i, "Unknown command")
        return
    }

    // Check permissions
    if !CheckCommandPermissions(s, i) {
        respondWithError(s, i, "You don't have permission to use this command")
        return
    }

    // Execute handler
    handler(s, i)
}

// Helper functions

func CheckCommandPermissions(s *discordgo.Session, i *discordgo.InteractionCreate) bool {
    // Admin commands require admin permissions
    if i.ApplicationCommandData().Name == "admin" {
        return hasAdminPermission(s, i)
    }

    // Source management commands require manage server permission
    if i.ApplicationCommandData().Name == "source" {
        return hasManageServerPermission(s, i)
    }

    return true
}

func hasAdminPermission(s *discordgo.Session, i *discordgo.InteractionCreate) bool {
    // Check if user is bot owner
    if i.Member.User.ID == cfg.OwnerID {
        return true
    }

    // Check if user has admin permission
    return i.Member.Permissions&discordgo.PermissionAdministrator != 0
}

func hasManageServerPermission(s *discordgo.Session, i *discordgo.InteractionCreate) bool {
    return i.Member.Permissions&discordgo.PermissionManageServer != 0
}

func respondWithError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
    s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseChannelMessageWithSource,
        Data: &discordgo.InteractionResponseData{
            Content: "❌ " + message,
            Flags:   discordgo.MessageFlagsEphemeral,
        },
    })
}
