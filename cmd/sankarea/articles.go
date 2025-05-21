package main

import (
    "fmt"
    "log"
    "time"

    "github.com/bwmarrin/discordgo"
    "github.com/mmcdole/gofeed"
    "github.com/robfig/cron/v3"
)

// Post top article summaries every 2 hours from curated sources
func PostTopArticles(dg *discordgo.Session, channelID string, sources []Source) {
    sourcesLock.Lock()
    defer sourcesLock.Unlock()

    if state.Paused {
        return
    }

    fp := gofeed.NewParser()
    postedSources := 0

    for _, src := range sources {
        if !src.Active {
            continue
        }
        // Filter for gov or main political biases for article posting
        if src.Bias != "gov" && src.Bias != "left" && src.Bias != "right" && src.Bias != "center" {
            continue
        }

        feed, err := fp.ParseURL(src.URL)
        if err != nil {
            log.Printf("Article fetch failed for %s: %v", src.Name, err)
            continue
        }
        if len(feed.Items) == 0 {
            continue
        }

        topItem := feed.Items[0]
        msg := fmt.Sprintf("**Top from [%s] (%s):**\n[%s](%s)", src.Name, src.Bias, topItem.Title, topItem.Link)
        _, err = dg.ChannelMessageSend(channelID, msg)
        if err != nil {
            log.Printf("Failed to post article from %s: %v", src.Name, err)
        } else {
            postedSources++
        }
    }
    log.Printf("Posted top articles from %d sources", postedSources)
}

// Schedule article posting every 2 hours
func ScheduleArticlePosting(cronJob *cron.Cron, dg *discordgo.Session, channelID string, sources []Source) {
    _, err := cronJob.AddFunc("@every 2h", func() {
        PostTopArticles(dg, channelID, sources)
    })
    if err != nil {
        LogAudit("CronError", fmt.Sprintf("Failed to schedule article posting: %v", err), 0xff0000)
    }
}
