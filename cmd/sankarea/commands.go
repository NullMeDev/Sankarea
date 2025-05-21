package main

import (
    "log"

    "github.com/bwmarrin/discordgo"
)

// List of slash commands to register
var commands = []*discordgo.ApplicationCommand{
    {
        Name:        "ping",
        Description: "Replies with Pong!",
    },
    {
        Name:        "status",
        Description: "Shows bot status and info",
    },
    {
        Name:        "setnewsinterval",
        Description: "Set news posting interval in minutes",
        Options: []*discordgo.ApplicationCommandOption{
            {
                Type:        discordgo.ApplicationCommandOptionInteger,
                Name:        "minutes",
                Description: "Interval in minutes (15-360)",
                Required:    true,
            },
        },
    },
    {
        Name:        "nullshutdown",
        Description: "Shutdown the bot (Admin only)",
    },
    {
        Name:        "nullrestart",
        Description: "Restart the bot (Admin only)",
    },
    {
        Name:        "silence",
        Description: "Silence a user for specified minutes (Admin only)",
        Options: []*discordgo.ApplicationCommandOption{
            {
                Type:        discordgo.ApplicationCommandOptionUser,
                Name:        "user",
                Description: "User to silence",
                Required:    true,
            },
            {
                Type:        discordgo.ApplicationCommandOptionInteger,
                Name:        "minutes",
                Description: "Duration in minutes",
                Required:    true,
            },
        },
    },
    {
        Name:        "unsilence",
        Description: "Remove silence from a user (Admin only)",
        Options: []*discordgo.ApplicationCommandOption{
            {
                Type:        discordgo.ApplicationCommandOptionUser,
                Name:        "user",
                Description: "User to unsilence",
                Required:    true,
            },
        },
    },
    {
        Name:        "kick",
        Description: "Kick a user from the server (Admin only)",
        Options: []*discordgo.ApplicationCommandOption{
            {
                Type:        discordgo.ApplicationCommandOptionUser,
                Name:        "user",
                Description: "User to kick",
                Required:    true,
            },
        },
    },
    {
        Name:        "ban",
        Description: "Ban a user from the server (Admin only)",
        Options: []*discordgo.ApplicationCommandOption{
            {
                Type:        discordgo.ApplicationCommandOptionUser,
                Name:        "user",
                Description: "User to ban",
                Required:    true,
            },
        },
    },
    {
        Name:        "factcheck",
        Description: "Check a claim against fact-check APIs",
        Options: []*discordgo.ApplicationCommandOption{
            {
                Type:        discordgo.ApplicationCommandOptionString,
                Name:        "claim",
                Description: "Claim text to check",
                Required:    true,
            },
        },
    },
    {
        Name:        "reloadconfig",
        Description: "Reload config and source lists (Admin only)",
    },
    {
        Name:        "health",
        Description: "Show bot health status",
    },
    {
        Name:        "uptime",
        Description: "Show bot uptime",
    },
    {
        Name:        "version",
        Description: "Show bot version",
    },
}

// Register all commands globally or per guild
func RegisterCommands(dg *discordgo.Session, guildID string) {
    for _, cmd := range commands {
        _, err := dg.ApplicationCommandCreate(dg.State.User.ID, guildID, cmd)
        if err != nil {
            log.Printf("Failed to create command %s: %v", cmd.Name, err)
        } else {
            log.Printf("Registered command: %s", cmd.Name)
        }
    }
}

// Unregister all commands (for cleanup or redeployment)
func UnregisterCommands(dg *discordgo.Session, guildID string) {
    cmds, err := dg.ApplicationCommands(dg.State.User.ID, guildID)
    if err != nil {
        log.Printf("Failed to list commands: %v", err)
        return
    }
    for _, cmd := range cmds {
        err := dg.ApplicationCommandDelete(dg.State.User.ID, guildID, cmd.ID)
        if err != nil {
            log.Printf("Failed to delete command %s: %v", cmd.Name, err)
        } else {
            log.Printf("Deleted command: %s", cmd.Name)
        }
    }
}
