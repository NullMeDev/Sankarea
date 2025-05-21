package main

import (
    "fmt"
    "time"

    "github.com/bwmarrin/discordgo"
)

// Silence a user for given minutes
func SilenceUser(s *discordgo.Session, i *discordgo.InteractionCreate) {
    if !isAdminOrOwner(i) {
        respondNoPrivilege(s, i)
        return
    }
    targetUser := i.ApplicationCommandData().Options[0].UserValue(s)
    mins := int(i.ApplicationCommandData().Options[1].IntValue())

    if !canTarget(i, targetUser.ID) {
        respondCantTarget(s, i)
        return
    }

    until := time.Now().Add(time.Duration(mins) * time.Minute)
    err := s.GuildMemberTimeout(i.GuildID, targetUser.ID, &until)
    if err != nil {
        respondFailure(s, i, "silence user", err)
        logAudit("SilenceFail", fmt.Sprintf("Attempt on <@%s> by <@%s>: %v", targetUser.ID, i.Member.User.ID, err), 0xff0000)
        return
    }
    logAudit("Silenced", fmt.Sprintf("<@%s> silenced for %d min by <@%s>", targetUser.ID, mins, i.Member.User.ID), 0xffcc00)
    s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseChannelMessageWithSource,
        Data: &discordgo.InteractionResponseData{
            Content: fmt.Sprintf("User <@%s> silenced for %d minutes.", targetUser.ID, mins),
        },
    })
}

// Unsilence user (remove timeout)
func UnsilenceUser(s *discordgo.Session, i *discordgo.InteractionCreate) {
    if !isAdminOrOwner(i) {
        respondNoPrivilege(s, i)
        return
    }
    targetUser := i.ApplicationCommandData().Options[0].UserValue(s)
    err := s.GuildMemberTimeout(i.GuildID, targetUser.ID, nil)
    if err != nil {
        respondFailure(s, i, "unsilence user", err)
        logAudit("UnsilenceFail", fmt.Sprintf("Attempt on <@%s> by <@%s>: %v", targetUser.ID, i.Member.User.ID, err), 0xff0000)
        return
    }
    logAudit("Unsilenced", fmt.Sprintf("<@%s> unsilenced by <@%s>", targetUser.ID, i.Member.User.ID), 0x00ff00)
    s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseChannelMessageWithSource,
        Data: &discordgo.InteractionResponseData{
            Content: fmt.Sprintf("User <@%s> unsilenced.", targetUser.ID),
        },
    })
}

// Kick a user
func KickUser(s *discordgo.Session, i *discordgo.InteractionCreate) {
    if !isAdminOrOwner(i) {
        respondNoPrivilege(s, i)
        return
    }
    targetUser := i.ApplicationCommandData().Options[0].UserValue(s)
    if !canTarget(i, targetUser.ID) {
        respondCantTarget(s, i)
        return
    }
    err := s.GuildMemberDeleteWithReason(i.GuildID, targetUser.ID, "Kicked by Sankarea bot")
    if err != nil {
        respondFailure(s, i, "kick user", err)
        logAudit("KickFail", fmt.Sprintf("Attempt on <@%s> by <@%s>: %v", targetUser.ID, i.Member.User.ID, err), 0xff0000)
        return
    }
    logAudit("Kicked", fmt.Sprintf("<@%s> kicked by <@%s>", targetUser.ID, i.Member.User.ID), 0xff6600)
    s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseChannelMessageWithSource,
        Data: &discordgo.InteractionResponseData{
            Content: fmt.Sprintf("User <@%s> kicked.", targetUser.ID),
        },
    })
}

// Ban a user
func BanUser(s *discordgo.Session, i *discordgo.InteractionCreate) {
    if !isAdminOrOwner(i) {
        respondNoPrivilege(s, i)
        return
    }
    targetUser := i.ApplicationCommandData().Options[0].UserValue(s)
    if !canTarget(i, targetUser.ID) {
        respondCantTarget(s, i)
        return
    }
    err := s.GuildBanCreateWithReason(i.GuildID, targetUser.ID, "Banned by Sankarea bot", 0)
    if err != nil {
        respondFailure(s, i, "ban user", err)
        logAudit("BanFail", fmt.Sprintf("Attempt on <@%s> by <@%s>: %v", targetUser.ID, i.Member.User.ID, err), 0xff0000)
        return
    }
    logAudit("Banned", fmt.Sprintf("<@%s> banned by <@%s>", targetUser.ID, i.Member.User.ID), 0xff0000)
    s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseChannelMessageWithSource,
        Data: &discordgo.InteractionResponseData{
            Content: fmt.Sprintf("User <@%s> banned.", targetUser.ID),
        },
    })
}

// Helper responses
func respondNoPrivilege(s *discordgo.Session, i *discordgo.InteractionCreate) {
    s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseChannelMessageWithSource,
        Data: &discordgo.InteractionResponseData{
            Content: "Weeb, You Do Not Have The Right Privileges.",
            Flags:   1 << 6,
        },
    })
}

func respondCantTarget(s *discordgo.Session, i *discordgo.InteractionCreate) {
    s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseChannelMessageWithSource,
        Data: &discordgo.InteractionResponseData{
            Content: "Cannot perform action on a user with equal or higher permissions.",
            Flags:   1 << 6,
        },
    })
}

func respondFailure(s *discordgo.Session, i *discordgo.InteractionCreate, action string, err error) {
    s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseChannelMessageWithSource,
        Data: &discordgo.InteractionResponseData{
            Content: fmt.Sprintf("Failed to %s: %v", action, err),
            Flags:   1 << 6,
        },
    })
}
