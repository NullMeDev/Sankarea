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
	articlesLock sync.Mutex
	articlesSent = make(map[string]bool)
)

// fetchAndPostArticles fetches and posts new articles from active sources to the Discord channel
func fetchAndPostArticles(dg *discordgo.Session, channelID string, sources []Source, maxPosts int) {
	articlesLock.Lock()
	defer articlesLock.Unlock()

	if state.Paused {
		return
	}

	fp := gofeed.NewParser()
	postedCount := 0

	for _, src := range sources {
		if !src.Active {
			continue
		}

		feed, err := fp.ParseURL(src.URL)
		if err != nil {
			log.Printf("Failed to fetch feed %s: %v", src.Name, err)
			continue
		}

		for _, item := range feed.Items {
			if postedCount >= maxPosts {
				break
			}
			if articlesSent[item.GUID] {
				continue
			}

			msg := fmt.Sprintf("**[%s]** *(bias: %s)*\n[%s](%s)", src.Name, src.Bias, item.Title, item.Link)
			_, err := dg.ChannelMessageSend(channelID, msg)
			if err != nil {
				log.Printf("Failed to post article from %s: %v", src.Name, err)
				continue
			}

			articlesSent[item.GUID] = true
			postedCount++
		}
	}

	log.Printf("Posted %d articles", postedCount)
	state.FeedCount = postedCount
	saveState(state)
}

// clearArticlesSent clears the map tracking sent articles (use on restart)
func clearArticlesSent() {
	articlesLock.Lock()
	defer articlesLock.Unlock()
	articlesSent = make(map[string]bool)
}
