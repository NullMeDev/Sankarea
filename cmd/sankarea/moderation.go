package main

import (
    "fmt"

    "github.com/bwmarrin/discordgo"
)

// isAdminOrOwner stub—allow all for now
func isAdminOrOwner(s *discordgo.Session, i *discordgo.InteractionCreate) bool {
    return true
}

// handleInteraction routes slash commands
func handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
    if !isAdminOrOwner(s, i) {
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{Content: "Permission denied."},
        })
        return
    }

    name := i.ApplicationCommandData().Name
    switch name {
    case "ping":
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{Content: "Pong!"},
        })
    case "status":
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{Content: "Bot is running normally."},
        })
    case "version":
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: fmt.Sprintf("Version: %s", cfg.Version),
            },
        })
    default:
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{Content: "Unknown command."},
        })
    }
}
