// cmd/sankarea/commands.go
package main

import (
    "fmt"
    "strings"
    "time"

    "github.com/bwmarrin/discordgo"
)

// CommandHandler represents a slash command handler
type CommandHandler struct {
    Name        string
    Description string
    Options     []*discordgo.ApplicationCommandOption
    Handler     func(s *discordgo.Session, i *discordgo.InteractionCreate)
    Permission  int // Using constants defined earlier (PermLevelEveryone, PermLevelAdmin, etc.)
}

var commands = []*discordgo.ApplicationCommand{
    {
        Name:        "news",
        Description: "News related commands",
        Options: []*discordgo.ApplicationCommandOption{
            {
                Type:        discordgo.ApplicationCommandOptionSubCommand,
                Name:        "fetch",
                Description: "Manually fetch news from sources",
            },
            {
                Type:        discordgo.ApplicationCommandOptionSubCommand,
                Name:        "sources",
                Description: "List all news sources",
            },
            {
                Type:        discordgo.ApplicationCommandOptionSubCommand,
                Name:        "pause",
                Description: "Pause news fetching",
            },
            {
                Type:        discordgo.ApplicationCommandOptionSubCommand,
                Name:        "resume",
                Description: "Resume news fetching",
            },
        },
    },
    {
        Name:        "status",
        Description: "Show bot status and statistics",
    },
    {
        Name:        "digest",
        Description: "Generate a news digest",
        Options: []*discordgo.ApplicationCommandOption{
            {
                Type:        discordgo.ApplicationCommandOptionString,
                Name:        "period",
                Description: "Time period for digest (daily/weekly)",
                Required:    false,
                Choices: []*discordgo.ApplicationCommandOptionChoice{
                    {
                        Name:  "Daily",
                        Value: "daily",
                    },
                    {
                        Name:  "Weekly",
                        Value: "weekly",
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
                Name:        "lockdown",
                Description: "Enable/disable lockdown mode",
                Options: []*discordgo.ApplicationCommandOption{
                    {
                        Type:        discordgo.ApplicationCommandOptionBoolean,
                        Name:        "enable",
                        Description: "Enable or disable lockdown",
                        Required:    true,
                    },
                },
            },
        },
    },
}

var commandHandlers = map[string]*CommandHandler{
    "news": {
        Name:        "news",
        Description: "News related commands",
        Permission:  PermLevelEveryone,
        Handler:     handleNewsCommand,
    },
    "status": {
        Name:        "status",
        Description: "Show bot status and statistics",
        Permission:  PermLevelEveryone,
        Handler:     handleStatusCommand,
    },
    "digest": {
        Name:        "digest",
        Description: "Generate a news digest",
        Permission:  PermLevelEveryone,
        Handler:     handleDigestCommand,
    },
    "admin": {
        Name:        "admin",
        Description: "Administrative commands",
        Permission:  PermLevelAdmin,
        Handler:     handleAdminCommand,
    },
}

// registerCommands registers all slash commands
func registerCommands(s *discordgo.Session) error {
    Logger().Printf("Registering %d commands...", len(commands))

    // Remove existing commands first
    registeredCommands, err := s.ApplicationCommands(cfg.AppID, cfg.GuildID)
    if err != nil {
        return fmt.Errorf("failed to fetch registered commands: %v", err)
    }

    for _, cmd := range registeredCommands {
        if err := s.ApplicationCommandDelete(cfg.AppID, cfg.GuildID, cmd.ID); err != nil {
            Logger().Printf("Failed to delete command %s: %v", cmd.Name, err)
        }
    }

    // Register new commands
    for _, cmd := range commands {
        _, err := s.ApplicationCommandCreate(cfg.AppID, cfg.GuildID, cmd)
        if err != nil {
            return fmt.Errorf("failed to register command %s: %v", cmd.Name, err)
        }
    }

    Logger().Printf("Successfully registered %d commands", len(commands))
    return nil
}

// handleNewsCommand handles all news-related subcommands
func handleNewsCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
    options := i.ApplicationCommandData().Options
    if len(options) == 0 {
        respondWithError(s, i, "Invalid subcommand")
        return
    }

    subcommand := options[0].Name
    switch subcommand {
    case "fetch":
        handleNewsFetch(s, i)
    case "sources":
        handleNewsSources(s, i)
    case "pause":
        handleNewsPause(s, i)
    case "resume":
        handleNewsResume(s, i)
    default:
        respondWithError(s, i, "Unknown subcommand")
    }
}

// handleNewsFetch handles manual news fetching
func handleNewsFetch(s *discordgo.Session, i *discordgo.InteractionCreate) {
    // Acknowledge the command
    s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
    })

    // Start fetching
    if err := fetchNewsWithContext(context.Background()); err != nil {
        followupWithError(s, i, fmt.Sprintf("Failed to fetch news: %v", err))
        return
    }

    // Send success response
    s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
        Content: stringPtr("✅ News fetch completed successfully!"),
    })
}

// handleNewsSources lists all configured sources
func handleNewsSources(s *discordgo.Session, i *discordgo.InteractionCreate) {
    sources := loadSources()
    
    // Build sources list
    var activeCount, pausedCount int
    var content strings.Builder
    content.WriteString("📰 **News Sources**\n\n")

    for _, source := range sources {
        status := "🟢"
        if source.Paused {
            status = "🔴"
            pausedCount++
        } else {
            activeCount++
        }

        content.WriteString(fmt.Sprintf("%s **%s**\n", status, source.Name))
        if source.Description != "" {
            content.WriteString(fmt.Sprintf("└ %s\n", source.Description))
        }
    }

    content.WriteString(fmt.Sprintf("\n**Summary:** %d Active, %d Paused", activeCount, pausedCount))

    s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseChannelMessageWithSource,
        Data: &discordgo.InteractionResponseData{
            Content: content.String(),
        },
    })
}

// handleNewsPause pauses news fetching
func handleNewsPause(s *discordgo.Session, i *discordgo.InteractionCreate) {
    if err := UpdateState(func(s *State) {
        s.Paused = true
        s.PausedBy = i.Member.User.Username
    }); err != nil {
        respondWithError(s, i, "Failed to update state")
        return
    }

    s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseChannelMessageWithSource,
        Data: &discordgo.InteractionResponseData{
            Content: "⏸️ News fetching has been paused",
        },
    })
}

// handleNewsResume resumes news fetching
func handleNewsResume(s *discordgo.Session, i *discordgo.InteractionCreate) {
    if err := UpdateState(func(s *State) {
        s.Paused = false
        s.PausedBy = ""
    }); err != nil {
        respondWithError(s, i, "Failed to update state")
        return
    }

    s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseChannelMessageWithSource,
        Data: &discordgo.InteractionResponseData{
            Content: "▶️ News fetching has been resumed",
        },
    })
}

// handleDigestCommand generates a news digest
func handleDigestCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
    // Acknowledge the command
    s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
    })

    period := "daily" // default
    if len(i.ApplicationCommandData().Options) > 0 {
        period = i.ApplicationCommandData().Options[0].StringValue()
    }

    var err error
    switch period {
    case "daily":
        err = GenerateDailyDigest(s, i.ChannelID)
    case "weekly":
        err = GenerateWeeklyDigest(s, i.ChannelID)
    default:
        err = fmt.Errorf("invalid digest period")
    }

    if err != nil {
        followupWithError(s, i, fmt.Sprintf("Failed to generate digest: %v", err))
        return
    }

    // Update digest stats
    UpdateState(func(s *State) {
        s.DigestCount++
        s.LastDigest = time.Now()
    })
}

// handleAdminCommand handles administrative commands
func handleAdminCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
    if !isAdmin(i.Member.User.ID) {
        respondWithError(s, i, "You don't have permission to use admin commands")
        return
    }

    options := i.ApplicationCommandData().Options
    if len(options) == 0 {
        respondWithError(s, i, "Invalid subcommand")
        return
    }

    subcommand := options[0].Name
    switch subcommand {
    case "lockdown":
        handleLockdown(s, i)
    default:
        respondWithError(s, i, "Unknown admin subcommand")
    }
}

// handleLockdown handles lockdown mode
func handleLockdown(s *discordgo.Session, i *discordgo.InteractionCreate) {
    enable := i.ApplicationCommandData().Options[0].Options[0].BoolValue()

    if err := UpdateState(func(s *State) {
        s.Lockdown = enable
        s.LockdownSetBy = i.Member.User.Username
    }); err != nil {
        respondWithError(s, i, "Failed to update lockdown state")
        return
    }

    status := "enabled"
    if !enable {
        status = "disabled"
    }

    s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseChannelMessageWithSource,
        Data: &discordgo.InteractionResponseData{
            Content: fmt.Sprintf("🔒 Lockdown mode %s", status),
        },
    })
}

// isAdmin checks if a user has admin permissions
func isAdmin(userID string) bool {
    for _, id := range cfg.OwnerIDs {
        if id == userID {
            return true
        }
    }
    return false
}
