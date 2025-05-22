package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mmcdole/gofeed"
)

var (
	sourcesLock sync.Mutex
)

// fetchAndPostNews fetches active RSS sources and posts latest items to Discord channel
func fetchAndPostNews(dg *discordgo.Session, channelID string, sources []Source) {
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
			logAudit("FeedError", fmt.Sprintf("Failed to fetch %s: %v", src.Name, err), 0xff0000)
			continue
		}
		if len(feed.Items) == 0 {
			continue
		}
		msg := fmt.Sprintf("**[%s]** *(bias: %s)*\n[%s](%s)", src.Name, src.Bias, feed.Items[0].Title, feed.Items[0].Link)
		_, err = dg.ChannelMessageSend(channelID, msg)
		if err != nil {
			logAudit("PostError", fmt.Sprintf("Failed to post %s: %v", src.Name, err), 0xff0000)
		} else {
			posted++
		}
	}
	state.NewsNextTime = time.Now().Add(parseCron(currentConfig.News15MinCron))
	state.FeedCount = posted
	saveState(state)
}

// postNewsDigest posts a digest of top news items from all active sources
func postNewsDigest(dg *discordgo.Session, channelID string, sources []Source) {
	if state.Paused {
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:     "News Digest",
		Color:     0x0099ff,
		Timestamp: time.Now().Format(time.RFC3339),
		Fields:    []*discordgo.MessageEmbedField{},
	}

	fp := gofeed.NewParser()
	for _, src := range sources {
		if !src.Active {
			continue
		}
		feed, err := fp.ParseURL(src.URL)
		if err != nil {
			continue
		}
		if len(feed.Items) > 0 {
			title := fmt.Sprintf("%s (%s)", src.Name, src.Bias)
			value := fmt.Sprintf("[%s](%s)", feed.Items[0].Title, feed.Items[0].Link)
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   title,
				Value:  value,
				Inline: false,
			})
		}
	}

	if len(embed.Fields) > 0 {
		_, err := dg.ChannelMessageSendEmbed(channelID, embed)
		if err != nil {
			logAudit("DigestError", fmt.Sprintf("Failed to post digest: %v", err), 0xff0000)
		} else {
			state.LastDigest = time.Now()
			saveState(state)
		}
	}
}

func parseCron(cronSpec string) time.Duration {
	var mins int
	_, err := fmt.Sscanf(cronSpec, "*/%d * * * *", &mins)
	if err != nil || mins < 15 {
		return 15 * time.Minute
	}
	return time.Duration(mins) * time.Minute
}

func updateCronJob(minutes int) {
	if cronJob != nil && cronJobID != 0 {
		cronJob.Remove(cronJobID)
	}
	spec := fmt.Sprintf("*/%d * * * *", minutes)
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
