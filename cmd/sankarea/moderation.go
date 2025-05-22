package main

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
)

// canTarget checks if the command issuer can moderate the target user (no equal or higher roles)
func canTarget(i *discordgo.InteractionCreate, targetID string) bool {
	if targetID == discordOwnerID {
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

// silenceUser applies timeout (mute) to a user for specified minutes
func silenceUser(s *discordgo.Session, i *discordgo.InteractionCreate, targetUserID string, minutes int) error {
	until := time.Now().Add(time.Duration(minutes) * time.Minute)
	return s.GuildMemberTimeout(i.GuildID, targetUserID, &until)
}

// unsilenceUser removes timeout from a user
func unsilenceUser(s *discordgo.Session, i *discordgo.InteractionCreate, targetUserID string) error {
	return s.GuildMemberTimeout(i.GuildID, targetUserID, nil)
}

// kickUser kicks a user from the guild
func kickUser(s *discordgo.Session, i *discordgo.InteractionCreate, targetUserID string) error {
	return s.GuildMemberDeleteWithReason(i.GuildID, targetUserID, "Kicked by admin/owner via Sankarea bot")
}

// banUser bans a user from the guild
func banUser(s *discordgo.Session, i *discordgo.InteractionCreate, targetUserID string) error {
	return s.GuildBanCreateWithReason(i.GuildID, targetUserID, "Banned by admin/owner via Sankarea bot", 0)
}
