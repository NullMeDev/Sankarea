package main

import (
	"github.com/bwmarrin/discordgo"
)

// CheckPermissionLevel checks if the user has the required permission level
// Returns: permission level the user has (0=everyone, 1=admin, 2=owner)
func CheckPermissionLevel(s *discordgo.Session, i *discordgo.InteractionCreate) int {
	// Check if user is an owner
	userID := i.Member.User.ID
	for _, ownerID := range cfg.OwnerIDs {
		if userID == ownerID {
			return PermLevelOwner
		}
	}

	// Check if user has admin role
	for _, userRole := range i.Member.Roles {
		for _, adminRole := range cfg.AdminRoleIDs {
			if userRole == adminRole {
				return PermLevelAdmin
			}
		}
	}

	// Default permission level
	return PermLevelEveryone
}

// IsAdmin checks if the user is an admin or owner
func IsAdmin(s *discordgo.Session, i *discordgo.InteractionCreate) bool {
	return CheckPermissionLevel(s, i) >= PermLevelAdmin
}

// IsOwner checks if the user is an owner
func IsOwner(s *discordgo.Session, i *discordgo.InteractionCreate) bool {
	return CheckPermissionLevel(s, i) >= PermLevelOwner
}

// CheckCommandPermissions checks if the user has permission to use the command
func CheckCommandPermissions(s *discordgo.Session, i *discordgo.InteractionCreate) bool {
	// Get command name
	if i.Type != discordgo.InteractionApplicationCommand {
		return false
	}

	cmd := i.ApplicationCommandData().Name

	// Check if command requires special permissions
	if CommandRequiresOwner(cmd) && !IsOwner(s, i) {
		return false
	}

	if CommandRequiresAdmin(cmd) && !IsAdmin(s, i) {
		return false
	}

	// Special handling for admin subcommands
	if cmd == "admin" {
		// All admin subcommands require at least admin permission
		if !IsAdmin(s, i) {
			return false
		}

		// Some admin subcommands require owner permission
		subCmd := i.ApplicationCommandData().Options[0].Name
		if subCmd == "config" && !IsOwner(s, i) {
			return false
		}
	}

	return true
}

// AuditLog logs admin actions to the audit log channel
func AuditLog(s *discordgo.Session, action, userID, details string) {
	if cfg.AuditLogChannelID == "" {
		return
	}

	// Format: 📝 **Action**: User (<@ID>) performed action - details
	message := "📝 **" + action + "**: <@" + userID + "> - " + details
	_, err := s.ChannelMessageSend(cfg.AuditLogChannelID, message)
	if err != nil {
		Logger().Printf("Failed to log audit message: %v", err)
	}
}
