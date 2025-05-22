package main

import (
    "fmt"
    "strconv"
    "strings"
    "time"

    "github.com/bwmarrin/discordgo"
    "github.com/mmcdole/gofeed"
    "github.com/robfig/cron/v3"
)

// fetchAndPostNews retrieves each feed and posts a simple report
func fetchAndPostNews(s *discordgo.Session, channelID string, sources []Source) {
    parser := gofeed.NewParser()
    for _, src := range sources {
        if src.Paused {
            continue
        }
        feed, err := parser.ParseURL(src.URL)
        if err != nil {
            Logger().Printf("fetch %s failed: %v", src.Name, err)
            continue
        }
        msg := fmt.Sprintf("🔗 %s: %s (%d items)", src.Name, feed.Title, len(feed.Items))
        s.ChannelMessageSend(channelID, msg)
    }
}

// postNewsDigest could aggregate and post a daily digest
func postNewsDigest(s *discordgo.Session, channelID string, sources []Source) {
    header := fmt.Sprintf("📰 Daily Digest (%s)", time.Now().Format("2006-01-02"))
    s.ChannelMessageSend(channelID, header)
    fetchAndPostNews(s, channelID, sources)
}

// parseCron turns a "*/N * * * *" spec into minutes
func parseCron(cronSpec string) time.Duration {
    // look for "/N"
    parts := strings.Split(cronSpec, "/")
    if len(parts) >= 2 {
        nPart := strings.Fields(parts[1])[0]
        if n, err := strconv.Atoi(nPart); err == nil {
            return time.Duration(n) * time.Minute
        }
    }
    // fallback
    return 15 * time.Minute
}

// UpdateCronJob installs or updates the 15-min job
func UpdateCronJob(c *cron.Cron, entryID *cron.EntryID, minutes int, dg *discordgo.Session, channelID string, sources []Source) {
    if entryID != nil {
        c.Remove(*entryID)
    }
    spec := fmt.Sprintf("@every %dm", minutes)
    id, err := c.AddFunc(spec, func() {
        fetchAndPostNews(dg, channelID, sources)
    })
    if err != nil {
        Logger().Printf("cron [%s] failed: %v", spec, err)
    } else {
        *entryID = id
    }
}
