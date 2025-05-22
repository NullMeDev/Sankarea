package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/bwmarrin/discordgo"
)

// registerCommands registers all slash commands used by the bot.
func registerCommands(s *discordgo.Session, appID, guildID string) error {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "ping",
			Description: "Check if the bot is alive",
		},
		{
			Name:        "status",
			Description: "Show bot status and news posting info",
		},
		{
			Name:        "setnewsinterval",
			Description: "Set how often news posts (in minutes)",
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
			Description: "Set how often news digests post (in hours)",
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
			Name:                     "nullshutdown",
			Description:              "Shut down the bot (admin only)",
			DefaultMemberPermissions: &[]int64{discordgo.PermissionAdministrator}[0],
		},
		{
			Name:                     "nullrestart",
			Description:              "Restart the bot (admin only)",
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
			Name:                     "unsilence",
			Description:              "Remove timeout from a user (admin only)",
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
			Name:                     "kick",
			Description:              "Kick a user (admin only)",
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
			Name:                     "ban",
			Description:              "Ban a user (admin only)",
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
			Name:                     "reloadconfig",
			Description:              "Reload bot config (admin only)",
			DefaultMemberPermissions: &[]int64{discordgo.PermissionAdministrator}[0],
		},
		{
			Name:        "health",
			Description: "Check bot health status",
		},
		{
			Name:        "version",
			Description: "Show bot version",
		},
	}

	for _, cmd := range commands {
		_, err := s.ApplicationCommandCreate(appID, guildID, cmd)
		if err != nil {
			return fmt.Errorf("failed to create command '%s': %w", cmd.Name, err)
		}
	}
	return nil
}

// handleCommands processes incoming slash commands
func handleCommands(s *discordgo.Session, i *discordgo.InteractionCreate) {
	defer logPanic()
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

	if i.GuildID == "" && (name == "kick" || name == "ban" || name == "nullrestart" || name == "nullshutdown" || name == "setnewsinterval" || name == "lockdown" || name == "unlock" || name == "silence" || name == "unsilence") {
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
	case "status", "uptime":
		paused := "No"
		if state.Paused {
			paused = "Yes"
		}
		summary := fmt.Sprintf("News posting paused: **%s**\nFeeds enabled: **%d**\nCurrent interval: **%d minutes**\nNext post: **%s**\nLockdown: **%v**\nUptime: **%s**",
			paused, state.FeedCount, state.LastInterval, state.NewsNextTime.Format(time.RFC1123), state.Lockdown, getUptime())
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: summary,
			},
		})
	case "setnewsinterval":
		if !isAdminOrOwner(i) {
			respondNoPrivilege(s, i)
			return
		}
		mins := int(i.ApplicationCommandData().Options[0].IntValue())
		if mins < 15 || mins > 360 {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Interval must be between 15 and 360 minutes.",
					Flags:   1 << 6,
				},
			})
			return
		}
		updateCronJob(mins)
		logAudit("IntervalChange", fmt.Sprintf("By <@%s>: Now every %d min", userID, mins), 0xffcc00)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("News interval updated to %d minutes.", mins),
			},
		})
	case "nullshutdown":
		if !isAdminOrOwner(i) {
			respondNoPrivilege(s, i)
			return
		}
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Shutting down bot. Goodbye.",
			},
		})
		logAudit("Shutdown", fmt.Sprintf("Shutdown requested by <@%s>", userID), 0xff0000)
		go func() {
			time.Sleep(2 * time.Second)
			os.Exit(0)
		}()
	case "nullrestart":
		if !isAdminOrOwner(i) {
			respondNoPrivilege(s, i)
			return
		}
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Restarting bot...",
			},
		})
		logAudit("Restart", fmt.Sprintf("Restart requested by <@%s>", userID), 0xffcc00)
		go func() {
			time.Sleep(2 * time.Second)
			os.Exit(42) // Runner script handles this as restart signal
		}()
	// Additional commands: silence, unsilence, kick, ban, factcheck, reloadconfig, health, version, default
	default:
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Unknown or unimplemented command.",
			},
		})
	}
}

func respondNoPrivilege(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "You do not have permission to use this command.",
			Flags:   1 << 6,
		},
	})
}

func isAdminOrOwner(i *discordgo.InteractionCreate) bool {
	if i.GuildID != "" && discordOwnerID != "" && i.Member.User.ID == discordOwnerID {
		return true
	}
	const adminPerm = 0x00000008
	return i.Member.Permissions&adminPerm == adminPerm
}

var cooldowns = make(map[string]time.Time)

func enforceCooldown(userID, command string) bool {
	key := userID + "|" + command
	last, ok := cooldowns[key]
	if ok && time.Since(last) < 10*time.Second {
		return false
	}
	cooldowns[key] = time.Now()
	return true
}

func logPanic() {
	if r := recover(); r != nil {
		msg := fmt.Sprintf("PANIC: %v", r)
		log.Println(msg)
		if dg != nil && auditLogChannelID != "" {
			embed := &discordgo.MessageEmbed{
				Title:       "Critical Panic",
				Description: msg,
				Color:       0xff0000,
				Timestamp:   time.Now().Format(time.RFC3339),
			}
			_, _ = dg.ChannelMessageSendEmbed(auditLogChannelID, embed)
		}
	}
}

func logAudit(action, details string, color int) {
	if auditLogChannelID == "" || dg == nil {
		return
	}
	embed := &discordgo.MessageEmbed{
		Title:       action,
		Description: details,
		Color:       color,
		Timestamp:   time.Now().Format(time.RFC3339),
	}
	_, _ = dg.ChannelMessageSendEmbed(auditLogChannelID, embed)
}

func updateCronJob(minutes int) {
	if cronJob != nil && cronJobID != 0 {
		cronJob.Remove(cronJobID)
	}
	spec := fmt.Sprintf("*/%d * * * *", minutes)
	id spec := fmt.Sprintf("*/%d * * * *", minutes)
	id, err := cronJob.AddFunc(spec, func() {
		fetchAndPostNews(dg, discordChannelID, sources)
	})
	if err != nil {
		logAudit("CronError", fmt.Sprintf("Failed to update cron job: %v", err), 0xff0000)
		return
	}
	cronJobID = id
	currentConfig.News15MinCron = spec
	state.LastInterval = minutes
	saveConfig(currentConfig)
	saveState(state)
}

// getUptime returns the bot uptime as a formatted string
func getUptime() string {
	return time.Since(startTime).Truncate(time.Second).String()
}

var startTime = time.Now()
