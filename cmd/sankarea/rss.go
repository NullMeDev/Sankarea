package main

import (
    "fmt"
    "log"
    "time"

    "github.com/bwmarrin/discordgo"
    "github.com/mmcdole/gofeed"
    "github.com/robfig/cron/v3"
)

// Fetch and post the latest RSS news items to Discord
func FetchAndPostNews(dg *discordgo.Session, channelID string, sources []Source) {
    sourcesLock.Lock()
    defer sourcesLock.Unlock()

    if state.Paused {
        return
    }

    fp := gofeed.NewParser()
    posted := 0

    for _, src := range sources {
        if !src.Active {
            continue
        }

        feed, err := fp.ParseURL(src.URL)
        if err != nil {
            LogAudit("FeedError", fmt.Sprintf("Failed to fetch %s: %v", src.Name, err), 0xff0000)
            continue
        }
        if len(feed.Items) == 0 {
            continue
        }

        msg := fmt.Sprintf("**[%s]** *(bias: %s)*\n[%s](%s)", src.Name, src.Bias, feed.Items[0].Title, feed.Items[0].Link)
        _, err = dg.ChannelMessageSend(channelID, msg)
        if err != nil {
            LogAudit("PostError", fmt.Sprintf("Failed to post %s: %v", src.Name, err), 0xff0000)
        } else {
            posted++
        }
    }

    state.NewsNextTime = time.Now().Add(parseCron(currentConfig.News15MinCron))
    state.FeedCount = posted
    if err := SaveState(state); err != nil {
        log.Printf("Failed to save state: %v", err)
    }
}

// Convert cron spec like "*/15 * * * *" to time.Duration
func parseCron(cronSpec string) time.Duration {
    var mins int
    _, err := fmt.Sscanf(cronSpec, "*/%d * * * *", &mins)
    if err != nil || mins < 15 {
        return 15 * time.Minute
    }
    return time.Duration(mins) * time.Minute
}

// Schedule the RSS feed posting cron job with the given interval in minutes
func ScheduleRSSPosting(cronJob *cron.Cron, dg *discordgo.Session, channelID string, sources []Source) {
    var minutes int
    _, err := fmt.Sscanf(currentConfig.News15MinCron, "*/%d * * * *", &minutes)
    if err != nil || minutes < 15 {
        minutes = 15
    }

    if cronJob != nil {
        cronJob.Stop()
    }

    cronJob = cron.New()
    _, err = cronJob.AddFunc(fmt.Sprintf("*/%d * * * *", minutes), func() {
        FetchAndPostNews(dg, channelID, sources)
    })
    if err != nil {
        LogAudit("CronError", fmt.Sprintf("Failed to schedule RSS feed: %v", err), 0xff0000)
        return
    }
    cronJob.Start()
}
