package main

import (
    "fmt"
    "github.com/bwmarrin/discordgo"
)

// RegisterCommands creates all slash commands for the bot
func RegisterCommands(s *discordgo.Session, appID, guildID string) error {
    commands := []*discordgo.ApplicationCommand{
        {
            Name:        "ping",
            Description: "Check if the bot is alive",
        },
        {
            Name:        "status",
            Description: "Show bot status and news posting information",
        },
        {
            Name:        "setnewsinterval",
            Description: "Set how often news is posted (in minutes)",
            Options: []*discordgo.ApplicationCommandOption{
                {
                    Type:        discordgo.ApplicationCommandOptionInteger,
                    Name:        "minutes",
                    Description: "Minutes between posts (15-360)",
                    Required:    true,
                    MinValue:    &[]float64{15}[0],
                    MaxValue:    360,
                },
            },
        },
        {
            Name:        "setdigestinterval",
            Description: "Set how often news digests are posted (in hours)",
            Options: []*discordgo.ApplicationCommandOption{
                {
                    Type:        discordgo.ApplicationCommandOptionInteger,
                    Name:        "hours",
                    Description: "Hours between digests (1-24)",
                    Required:    true,
                    MinValue:    &[]float64{1}[0],
                    MaxValue:    24,
                },
            },
            DefaultMemberPermissions: &[]int64{discordgo.PermissionAdministrator}[0],
        },
        {
            Name:        "nullshutdown",
            Description: "Shut down the bot (admin only)",
            DefaultMemberPermissions: &[]int64{discordgo.PermissionAdministrator}[0],
        },
        {
            Name:        "nullrestart",
            Description: "Restart the bot (admin only)",
            DefaultMemberPermissions: &[]int64{discordgo.PermissionAdministrator}[0],
        },
        {
            Name:        "silence",
            Description: "Timeout a user (admin only)",
            DefaultMemberPermissions: &[]int64{discordgo.PermissionModerateMembers}[0],
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
                    Description: "Minutes to silence for",
                    Required:    true,
                    MinValue:    &[]float64{1}[0],
                    MaxValue:    10080,
                },
            },
        },
        {
            Name:        "unsilence",
            Description: "Remove timeout from a user (admin only)",
            DefaultMemberPermissions: &[]int64{discordgo.PermissionModerateMembers}[0],
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
            Description: "Kick a user from the server (admin only)",
            DefaultMemberPermissions: &[]int64{discordgo.PermissionKickMembers}[0],
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
            Description: "Ban a user from the server (admin only)",
            DefaultMemberPermissions: &[]int64{discordgo.PermissionBanMembers}[0],
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
            Description: "Check if a claim is factual",
            Options: []*discordgo.ApplicationCommandOption{
                {
                    Type:        discordgo.ApplicationCommandOptionString,
                    Name:        "claim",
                    Description: "The claim to fact check",
                    Required:    true,
                },
            },
        },
        {
            Name:        "reloadconfig",
            Description: "Reload bot configuration (admin only)",
            DefaultMemberPermissions: &[]int64{discordgo.PermissionAdministrator}[0],
        },
        {
            Name:        "health",
            Description: "Check bot health status",
        },
        {
            Name:        "version",
            Description: "Show bot version information",
        },
    }

    for _, cmd := range commands {
        if _, err := s.ApplicationCommandCreate(appID, guildID, cmd); err != nil {
            return fmt.Errorf("failed to create '%s' command: %w", cmd.Name, err)
        }
    }
    return nil
}
