package main

import (
    "fmt"
    "sync"
    "time"

    "github.com/bwmarrin/discordgo"
)

var (
    cooldowns     = make(map[string]time.Time)
    cooldownMutex sync.Mutex
    cooldownDuration = 10 * time.Second
)

// Enforce per-user cooldown per command to avoid spam
func EnforceCooldown(userID, command string) bool {
    cooldownMutex.Lock()
    defer cooldownMutex.Unlock()

    key := userID + "|" + command
    last, ok := cooldowns[key]
    if ok && time.Since(last) < cooldownDuration {
        return false
    }
    cooldowns[key] = time.Now()
    return true
}

// Check if interaction user is owner or has admin permissions
func IsAdminOrOwner(i *discordgo.InteractionCreate) bool {
    if i.GuildID != "" && discordOwnerID != "" && i.Member.User.ID == discordOwnerID {
        return true
    }
    const adminPerm = 0x00000008
    return i.Member.Permissions&adminPerm == adminPerm
}

// Check if acting user can target the given target user (role hierarchy)
func CanTarget(i *discordgo.InteractionCreate, targetID string) bool {
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

// Parse cron style interval string like "*/15 * * * *" to time.Duration
func ParseCronInterval(cronSpec string) time.Duration {
    var mins int
    _, err := fmt.Sscanf(cronSpec, "*/%d * * * *", &mins)
    if err != nil || mins < 15 {
        return 15 * time.Minute
    }
    return time.Duration(mins) * time.Minute
}

// Get uptime string from start time to now
var startTime = time.Now()

func GetUptime() string {
    return time.Since(startTime).Truncate(time.Second).String()
}
