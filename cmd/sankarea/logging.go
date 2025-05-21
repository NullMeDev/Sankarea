package main

import (
    "fmt"
    "log"
    "time"

    "github.com/bwmarrin/discordgo"
)

// Panic recovery with audit log notification
func LogPanic() {
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

// Audit log sending to Discord audit channel
func LogAudit(action, details string, color int) {
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
