package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mmcdole/gofeed"
	"github.com/robfig/cron/v3"
	"github.com/joho/godotenv"
)

type Config struct {
	NewsCron          string `json:"newsCron"`
	AuditLogChannelID string `json:"auditLogChannelId"`
}

var (
	botSession    *discordgo.Session
	adminUserIDs  map[string]bool
	newsCronSpec  string
	newsTicker    *cron.Cron
	newsFeedURLs  = []string{
		"https://www.npr.org/rss/rss.php?id=1001",
		"https://www.theguardian.com/world/rss",
		"http://feeds.bbci.co.uk/news/rss.xml",
		"http://feeds.reuters.com/reuters/topNews",
		"https://apnews.com/rss",
		"https://www.pbs.org/newshour/feeds/rss/headlines",
		"http://rssfeeds.usatoday.com/usatoday-NewsTopStories",
		"http://feeds.foxnews.com/foxnews/latest",
		"https://www.washingtontimes.com/rss/headlines/news/",
		"https://nypost.com/feed/",
		"https://www.usa.gov/rss/updates.xml",
		"https://tools.cdc.gov/api/v2/resources/media/404952.rss",
		"https://www.nasa.gov/rss/dyn/breaking_news.rss",
	}
	config Config
	mu     sync.Mutex
)

func loadEnv() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Println("Warning: .env file not found or error loading it:", err)
	}
}

func loadConfig() {
	data, err := ioutil.ReadFile("config.json")
	if err != nil {
		log.Fatalf("Failed to read config.json: %v", err)
	}
	err = json.Unmarshal(data, &config)
	if err != nil {
		log.Fatalf("Failed to parse config.json: %v", err)
	}
	newsCronSpec = config.NewsCron
	if newsCronSpec == "" {
		newsCronSpec = "*/15 * * * *" // default every 15 minutes
	}
}

func initDiscord() {
	var err error
	botToken := os.Getenv("DISCORD_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("DISCORD_BOT_TOKEN environment variable is not set")
	}

	botSession, err = discordgo.New("Bot " + botToken)
	if err != nil {
		log.Fatalf("Failed to create Discord session: %v", err)
	}

	botSession.AddHandler(handleSlashCommands)

	err = botSession.Open()
	if err != nil {
		log.Fatalf("Failed to open Discord session: %v", err)
	}

	log.Println("Discord session started")
}

func setupAdminIDs() {
	adminUserIDs = map[string]bool{
		"YOUR_DISCORD_USER_ID": true, // Replace with your actual Discord User ID
		// Add more admin user IDs here as needed
	}
}

func isAdmin(userID string) bool {
	mu.Lock()
	defer mu.Unlock()
	return adminUserIDs[userID]
}

func handleSlashCommands(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	userID := i.Member.User.ID

	switch i.ApplicationCommandData().Name {
	case "setinterval":
		if !isAdmin(userID) {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Weeb, You Do Not Have The Right Privelages.",
				},
			})
			return
		}

		minutes := int(i.ApplicationCommandData().Options[0].IntValue())
		if minutes < 15 || minutes > 360 {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Interval must be between 15 and 360 minutes.",
				},
			})
			return
		}

		updateNewsInterval(minutes)

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("News posting interval updated to %d minutes.", minutes),
			},
		})

	// Extend with other commands here (mute, kick, ban, restart, shutdown, etc.)

	}
}

func updateNewsInterval(minutes int) {
	mu.Lock()
	defer mu.Unlock()
	newsCronSpec = fmt.Sprintf("*/%d * * * *", minutes)

	// Restart cron job with new schedule
	if newsTicker != nil {
		newsTicker.Stop()
	}

	newsTicker = cron.New()
	_, err := newsTicker.AddFunc(newsCronSpec, func() {
		postNews()
	})
	if err != nil {
		log.Printf("Failed to schedule news posting: %v", err)
		return
	}
	newsTicker.Start()

	// Update config.json file persistently
	config.NewsCron = newsCronSpec
	saveConfig()
}

func saveConfig() {
	b, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal config: %v", err)
		return
	}
	err = ioutil.WriteFile("config.json", b, 0644)
	if err != nil {
		log.Printf("Failed to write config.json: %v", err)
	}
}

func fetchNews() ([]*gofeed.Item, error) {
	fp := gofeed.NewParser()
	items := []*gofeed.Item{}

	for _, url := range newsFeedURLs {
		feed, err := fp.ParseURL(url)
		if err != nil {
			log.Printf("Failed to parse feed %s: %v", url, err)
			continue
		}
		if len(feed.Items) > 0 {
			items = append(items, feed.Items[0]) // latest article only
		}
	}

	return items, nil
}

func postNews() {
	channelID := os.Getenv("DISCORD_CHANNEL_ID")
	if channelID == "" {
		log.Println("DISCORD_CHANNEL_ID environment variable is not set")
		return
	}

	items, err := fetchNews()
	if err != nil {
		log.Println("Error fetching news:", err)
		return
	}

	for _, item := range items {
		msg := fmt.Sprintf("**%s**\n%s", item.Title, item.Link)
		_, err := botSession.ChannelMessageSend(channelID, msg)
		if err != nil {
			log.Printf("Failed to send message: %v", err)
		}
	}
}

func main() {
	loadEnv()
	loadConfig()
	setupAdminIDs()
	initDiscord()

	// Schedule news posting on startup
	updateNewsInterval(15) // default 15 minutes

	fmt.Println("Bot is running... Press CTRL+C to exit.")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	<-stop

	fmt.Println("Shutting down...")
	if newsTicker != nil {
		newsTicker.Stop()
	}
	if botSession != nil {
		botSession.Close()
	}
}
