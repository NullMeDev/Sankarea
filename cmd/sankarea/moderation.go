package main

import (
    "fmt"
    "time"

    "github.com/bwmarrin/discordgo"
)

// isAdminOrOwner checks if the user is the guild owner or has admin perms
func isAdminOrOwner(i *discordgo.InteractionCreate, ownerID string) bool {
    if i.GuildID != "" && i.Member.User.ID == ownerID {
        return true
    }
    const adminPerm = 0x00000008
    if i.Member.Permissions&adminPerm == adminPerm {
        return true
    }
    return false
}

// canTarget prevents moderating equal/higher roles
func canTarget(i *discordgo.InteractionCreate, dg *discordgo.Session, ownerID, targetID string) bool {
    if targetID == ownerID {
        return false
    }
    userRoles := i.Member.Roles
    member, err := dg.GuildMember(i.GuildID, targetID)
    if err != nil {
        return false
    }
    for _, rid := range member.Roles {
        for _, myrid := range userRoles {
            if rid == myrid {
                return false
            }
        }
    }
    return true
}

var cooldowns = make(map[string]time.Time)
const cooldownDuration = 10 * time.Second

// enforceCooldown ensures a user+command combo isn't spammed
func enforceCooldown(userID, command string) bool {
    k := userID + "|" + command
    last, ok := cooldowns[k]
    if ok && time.Since(last) < cooldownDuration {
        return false
    }
    cooldowns[k] = time.Now()
    return true
}

// HandleCommands routes incoming slash commands
func HandleCommands(s *discordgo.Session, i *discordgo.InteractionCreate, ownerID string) {
    name := i.ApplicationCommandData().Name
    userID := i.Member.User.ID

    if !enforceCooldown(userID, name) {
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: "Slow down. Try again in a moment.",
                Flags:   1 << 6,
            },
        })
        return
    }

    // Prevent dangerous commands in DM
    if i.GuildID == "" &&
        (name == "kick" || name == "ban" || name == "nullrestart" ||
            name == "nullshutdown" || name == "setnewsinterval" ||
            name == "silence" || name == "unsilence") {
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: "This command cannot be used in DM.",
                Flags:   1 << 6,
            },
        })
        return
    }

    switch name {
    case "ping":
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: "Pong!",
            },
        })
    // TODO: copy over other cases from main.go
    default:
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: "Unknown or unimplemented command.",
            },
        })
    }
}
