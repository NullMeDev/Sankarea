package main

import (
    "fmt"

    "github.com/bwmarrin/discordgo"
)

// RegisterCommands creates slash commands
func RegisterCommands(s *discordgo.Session, appID, guildID string) error {
    cmds := []*discordgo.ApplicationCommand{
        {
            Name:        "ping",
            Description: "Check if the bot is alive",
        },
        {
            Name:        "status",
            Description: "Show current status",
        },
        {
            Name:        "version",
            Description: "Show bot version information",
        },
    }

    for _, cmd := range cmds {
        if _, err := s.ApplicationCommandCreate(appID, guildID, cmd); err != nil {
            return fmt.Errorf("failed to create '%s': %w", cmd.Name, err)
        }
    }
    return nil
}
