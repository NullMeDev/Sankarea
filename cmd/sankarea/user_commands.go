package main

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
)

// RegisterUserManagementCommands registers all user management slash commands
func RegisterUserManagementCommands(s *discordgo.Session, appID, guildID string) error {
	cmds := []*discordgo.ApplicationCommand{
		{
			Name:        "kick",
			Description: "Kick a user from the server",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "The user to kick",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "reason",
					Description: "Reason for kicking the user",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "notify",
					Description: "Whether to send a DM to the user",
					Required:    false,
				},
			},
		},
		{
			Name:        "ban",
			Description: "Ban a user from the server",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "The user to ban",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "reason",
					Description: "Reason for banning the user",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "days",
					Description: "Number of days of messages to delete (0-7)",
					Required:    false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "Don't delete any messages", Value: 0},
						{Name: "Previous 24 hours", Value: 1},
						{Name: "Previous 7 days", Value: 7},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "notify",
					Description: "Whether to send a DM to the user",
					Required:    false,
				},
			},
		},
		{
			Name:        "mute",
			Description: "Timeout (mute) a user",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "The user to timeout",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "reason",
					Description: "Reason for the timeout",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "duration",
					Description: "Duration of the timeout in minutes",
					Required:    true,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "60 minutes", Value: 60},
						{Name: "1 day", Value: 1440},
						{Name: "1 week", Value: 10080},
						{Name: "28 days", Value: 40320},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "notify",
					Description: "Whether to send a DM to the user",
					Required:    false,
				},
			},
		},
		{
			Name:        "unmute",
			Description: "Remove timeout (unmute) a user",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "The user to unmute",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "reason",
					Description: "Reason for removing the timeout",
					Required:    true,
				},
			},
		},
	}

	for _, cmd := range cmds {
		_, err := s.ApplicationCommandCreate(appID, guildID, cmd)
		if err != nil {
			return err
		}
	}

	return nil
}

// HandleUserManagementCommands handles user management slash commands
func HandleUserManagementCommands(s *discordgo.Session, i *discordgo.InteractionCreate) bool {
	// Check if this is a user management command
	cmd := i.ApplicationCommandData().Name
	switch cmd {
	case "kick":
		handleKickCommand(s, i)
		return true
	case "ban":
		handleBanCommand(s, i)
		return true
	case "mute":
		handleMuteCommand(s, i)
		return true
	case "unmute":
		handleUnmuteCommand(s, i)
		return true
	default:
		return false
	}
}

// handleKickCommand handles the kick command
func handleKickCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Check permissions first
	if !IsAdmin(s, i) {
		respondWithError(s, i, "You don't have permission to use this command")
		return
	}

	options := i.ApplicationCommandData().Options
	var userID, reason string
	var notify bool = true // Default to true

	// Extract options
	for _, opt := range options {
		switch opt.Name {
		case "user":
			userID = opt.UserValue(s).ID
		case "reason":
			reason = opt.StringValue()
		case "notify":
			notify = opt.BoolValue()
		}
	}

	// Get the user's information
	member, err := s.GuildMember(i.GuildID, userID)
	if err != nil {
		respondWithError(s, i, "Failed to get user information")
		return
	}

	// Send DM notification before kicking if requested
	if notify {
		dmChannel, err := s.UserChannelCreate(userID)
		if err == nil {
			notificationMsg := fmt.Sprintf("You have been kicked from %s by %s for: %s", 
				i.GuildID, i.Member.User.Username, reason)
			s.ChannelMessageSend(dmChannel.ID, notificationMsg)
		}
	}

	// Kick the user
	err = s.GuildMemberDelete(i.GuildID, userID)
	if err != nil {
		respondWithError(s, i, "Failed to kick the user: "+err.Error())
		return
	}

	// Log to audit channel
	auditMessage := fmt.Sprintf("👢 **User Kicked**: %s#%s (ID: %s)\n**Reason**: %s\n**Performed by**: %s",
		member.User.Username, member.User.Discriminator, member.User.ID, reason, i.Member.User.Username)
	
	if cfg.AuditLogChannelID != "" {
		s.ChannelMessageSend(cfg.AuditLogChannelID, auditMessage)
	}

	// Respond to the command
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Successfully kicked %s#%s for: %s", 
				member.User.Username, member.User.Discriminator, reason),
		},
	})
}

// handleBanCommand handles the ban command
func handleBanCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Check permissions first
	if !IsAdmin(s, i) {
		respondWithError(s, i, "You don't have permission to use this command")
		return
	}

	options := i.ApplicationCommandData().Options
	var userID, reason string
	var days int
	var notify bool = true // Default to true

	// Extract options
	for _, opt := range options {
		switch opt.Name {
		case "user":
			userID = opt.UserValue(s).ID
		case "reason":
			reason = opt.StringValue()
		case "days":
			days = int(opt.IntValue())
		case "notify":
			notify = opt.BoolValue()
		}
	}

	// Get the user's information
	member, err := s.GuildMember(i.GuildID, userID)
	if err != nil {
		respondWithError(s, i, "Failed to get user information")
		return
	}

	// Send DM notification before banning if requested
	if notify {
		dmChannel, err := s.UserChannelCreate(userID)
		if err == nil {
			notificationMsg := fmt.Sprintf("You have been banned from %s by %s for: %s", 
				i.GuildID, i.Member.User.Username, reason)
			s.ChannelMessageSend(dmChannel.ID, notificationMsg)
		}
	}

	// Ban the user
	err = s.GuildBanCreateWithReason(i.GuildID, userID, reason, days)
	if err != nil {
		respondWithError(s, i, "Failed to ban the user: "+err.Error())
		return
	}

	// Log to audit channel
	auditMessage := fmt.Sprintf("🔨 **User Banned**: %s#%s (ID: %s)\n**Reason**: %s\n**Performed by**: %s\n**Days of messages deleted**: %d",
		member.User.Username, member.User.Discriminator, member.User.ID, reason, i.Member.User.Username, days)
	
	if cfg.AuditLogChannelID != "" {
		s.ChannelMessageSend(cfg.AuditLogChannelID, auditMessage)
	}

	// Respond to the command
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Successfully banned %s#%s for: %s", 
				member.User.Username, member.User.Discriminator, reason),
		},
	})
}

// handleMuteCommand handles the mute (timeout) command
func handleMuteCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Check permissions first
	if !IsAdmin(s, i) {
		respondWithError(s, i, "You don't have permission to use this command")
		return
	}

	options := i.ApplicationCommandData().Options
	var userID, reason string
	var duration int64
	var notify bool = true // Default to true

	// Extract options
	for _, opt := range options {
		switch opt.Name {
		case "user":
			userID = opt.UserValue(s).ID
		case "reason":
			reason = opt.StringValue()
		case "duration":
			duration = opt.IntValue()
		case "notify":
			notify = opt.BoolValue()
		}
	}

	// Get the user's information
	member, err := s.GuildMember(i.GuildID, userID)
	if err != nil {
		respondWithError(s, i, "Failed to get user information")
		return
	}

	// Calculate timeout duration
	timeoutUntil := time.Now().Add(time.Duration(duration) * time.Minute)

	// Send DM notification before muting if requested
	if notify {
		dmChannel, err := s.UserChannelCreate(userID)
		if err == nil {
			notificationMsg := fmt.Sprintf("You have been timed out in %s by %s for: %s. The timeout will expire in %d minutes.", 
				i.GuildID, i.Member.User.Username, reason, duration)
			s.ChannelMessageSend(dmChannel.ID, notificationMsg)
		}
	}

	// Apply timeout
	err = s.GuildMemberTimeoutWithReason(i.GuildID, userID, &timeoutUntil, reason)
	if err != nil {
		respondWithError(s, i, "Failed to timeout the user: "+err.Error())
		return
	}

	// Log to audit channel
	auditMessage := fmt.Sprintf("🔇 **User Timed Out**: %s#%s (ID: %s)\n**Reason**: %s\n**Duration**: %d minutes\n**Performed by**: %s",
		member.User.Username, member.User.Discriminator, member.User.ID, reason, duration, i.Member.User.Username)
	
	if cfg.AuditLogChannelID != "" {
		s.ChannelMessageSend(cfg.AuditLogChannelID, auditMessage)
	}

	// Respond to the command
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Successfully timed out %s#%s for %d minutes. Reason: %s", 
				member.User.Username, member.User.Discriminator, duration, reason),
		},
	})
}

// handleUnmuteCommand handles the unmute (remove timeout) command
func handleUnmuteCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Check permissions first
	if !IsAdmin(s, i) {
		respondWithError(s, i, "You don't have permission to use this command")
		return
	}

	options := i.ApplicationCommandData().Options
	var userID, reason string

	// Extract options
	for _, opt := range options {
		switch opt.Name {
		case "user":
			userID = opt.UserValue(s).ID
		case "reason":
			reason = opt.StringValue()
		}
	}

	// Get the user's information
	member, err := s.GuildMember(i.GuildID, userID)
	if err != nil {
		respondWithError(s, i, "Failed to get user information")
		return
	}

	// Remove timeout
	err = s.GuildMemberTimeoutWithReason(i.GuildID, userID, nil, reason)
	if err != nil {
		respondWithError(s, i, "Failed to remove timeout: "+err.Error())
		return
	}

	// Log to audit channel
	auditMessage := fmt.Sprintf("🔊 **User Timeout Removed**: %s#%s (ID: %s)\n**Reason**: %s\n**Performed by**: %s",
		member.User.Username, member.User.Discriminator, member.User.ID, reason, i.Member.User.Username)
	
	if cfg.AuditLogChannelID != "" {
		s.ChannelMessageSend(cfg.AuditLogChannelID, auditMessage)
	}

	// Respond to the command
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Successfully removed timeout for %s#%s. Reason: %s", 
				member.User.Username, member.User.Discriminator, reason),
		},
	})
}

// respondWithError responds with an error message
func respondWithError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

// GuildMemberTimeoutWithReason applies a timeout with a reason (helper since DiscordGo doesn't include this directly)
func (s *discordgo.Session) GuildMemberTimeoutWithReason(guildID, userID string, until *time.Time, reason string) error {
	data := struct {
		CommunicationDisabledUntil *time.Time `json:"communication_disabled_until,omitempty"`
		AuditLogReason             string     `json:"-"`
	}{
		CommunicationDisabledUntil: until,
		AuditLogReason:             reason,
	}

	_, err := s.RequestWithBucketID("PATCH", discordgo.EndpointGuildMember(guildID, userID), data, discordgo.EndpointGuildMember(guildID, ""))
	return err
}
