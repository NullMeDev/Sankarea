package main

import (
	"github.com/bwmarrin/discordgo"
)

// hasAdminPermissions checks if a user has admin permissions
func hasAdminPermissions(s *discordgo.Session, guildID, userID string) bool {
	// Get member
	member, err := s.GuildMember(guildID, userID)
	if err != nil {
		Logger().Printf("Error getting member: %v", err)
		return false
	}

	// Check for admin role
	for _, roleID := range member.Roles {
		role, err := s.State.Role(guildID, roleID)
		if err != nil {
			continue
		}
		
		// Check if the role has admin permissions
		if role.Permissions&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator {
			return true
		}
	}
	
	return false
}

// isOwner checks if the user is the bot owner
func isOwner(userID string) bool {
	// You can add owner IDs in the environment or config
	ownerID := os.Getenv("OWNER_ID")
	return ownerID == userID
}
