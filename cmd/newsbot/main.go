package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/bwmarrin/discordgo"
	openai "github.com/sashabaranov/go-openai"
	"gopkg.in/yaml.v3"
)

var dg *discordgo.Session

// Config holds feed & behavior settings
type Config struct {
	Keywords  []string
	FeedTypes map[string]struct {
		TTLDays            int     `yaml:"ttl_days"`
		SimilarityThreshold float64 `yaml:"similarity_threshold"`
	} `yaml:"feed_types"`
	Colors struct {
		Default    int `yaml:"default"`
		Government int `yaml:"government"`
	}
	RSS []struct {
		Name string   `yaml:"name"`
		URL  string   `yaml:"url"`
		Tags []string `yaml:"tags"`
	} `yaml:"rss"`
	HTML []struct {
		Name     string   `yaml:"name"`
		URL      string   `yaml:"url"`
		Selector string   `yaml:"selector"`
		Tags     []string `yaml:"tags"`
	} `yaml:"html"`
}

// Item represents a news article
type Item struct {
	Hash      string
	Title     string
	URL       string
	Tags      []string
	Summary   string
	Sentiment string
	Fetched   time.Time
	Body      string
	ThreadURL string
}

// State tracks seen items
type State struct {
	Seen map[string]time.Time `json:"seen"`
}

func main() {
	// Flags
	configPath := flag.String("config", "config/sources.yml", "")
	statePath := flag.String("state", "data/state.json", "")
	interval := flag.Int("interval", 24, "")
	maxItems := flag.Int("max", 6, "")
	flag.Parse()

	// Load config & state
	cfg := loadConfig(*configPath)
	st := loadState(*statePath)

	// Initialize Discord session for slash commands
	var err error
	dg, err = discordgo.New("Bot " + os.Getenv("DISCORD_BOT_TOKEN"))
	if err != nil {
		log.Fatalf("Discord session error: %v", err)
	}
	registerSlashCommands()
	if err := dg.Open(); err != nil {
		log.Fatalf("Discord WS open error: %v", err)
	}
	defer dg.Close()

	// OpenAI client
	oa := openai.NewClient(os.Getenv("OPENAI_API_KEY"))
	httpClient := &http.Client{Timeout: 20 * time.Second}

	// Continuous loop
	for {
		items := fetchAll(cfg, httpClient)
		items = filterNew(items, st)
		classify(items, oa)

		trending := pickTop(items, *maxItems)
		summaries := batchSummarize(trending, oa)
		for i := range trending {
			trending[i].Summary = summaries[i]
		}

		watch := filterKeywords(items, cfg.Keywords)
		embed := buildEmbed(trending, items, watch, cfg)
		postDiscord(embed)

		updateState(st, items)
		saveState(*statePath, st, items)
		archiveStaleThreads()

		time.Sleep(time.Duration(*interval) * time.Minute)
	}
}

// registerSlashCommands registers /addtag, /removetag, /listtags
func registerSlashCommands() {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "addtag",
			Description: "Add an alert tag",
			Options: []*discordgo.ApplicationCommandOption{{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "tag",
				Description: "Tag to add",
				Required:    true,
			}},
		},
		{
			Name:        "removetag",
			Description: "Remove an alert tag",
			Options: []*discordgo.ApplicationCommandOption{{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "tag",
				Description: "Tag to remove",
				Required:    true,
			}},
		},
		{
			Name:        "listtags",
			Description: "List current alert tags",
		},
	}

	for _, cmd := range commands {
		if _, err := dg.ApplicationCommandCreate(
			os.Getenv("DISCORD_APPLICATION_ID"),
			os.Getenv("DISCORD_GUILD_ID"),
			cmd,
		); err != nil {
			log.Printf("Cannot create '%s': %v", cmd.Name, err)
		}
	}

	dg.AddHandler(func(s *discordgo.Session, ev *discordgo.InteractionCreate) {
		data := ev.ApplicationCommandData()
		switch data.Name {
		case "addtag":
			tag := data.Options[0].StringValue()
			modifyAlertTags("addtag", tag)
			s.InteractionRespond(ev.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("✅ Added tag `%s`", tag),
				},
			})
		case "removetag":
			tag := data.Options[0].StringValue()
			modifyAlertTags("removetag", tag)
			s.InteractionRespond(ev.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("✅ Removed tag `%s`", tag),
				},
			})
		case "listtags":
			tags := getAlertTags()
			s.InteractionRespond(ev.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "⚙️ Tags: " + strings.Join(tags, ", "),
				},
			})
		}
	})
}

func loadConfig(path string) *Config {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalf("Read config: %v", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("Parse config: %v", err)
	}
	return &cfg
}

func loadState(path string) *State {
	st := &State{Seen: make(map[string]time.Time)}
	data, _ := ioutil.ReadFile(path)
	json.Unmarshal(data, st)
	return st
}

func saveState(path string, st *State, items []Item) {
	os.MkdirAll(filepath.Dir(path), 0755)

	// JSON state
	var articles []map[string]interface{}
	for _, it := range items {
		articles = append(articles, map[string]interface{}{
			"hash":       it.Hash,
			"title":      it.Title,
			"url":        it.URL,
			"tags":       it.Tags,
			"summary":    it.Summary,
			"sentiment":  it.Sentiment,
			"fetched":    it.Fetched,
			"thread_url": it.ThreadURL,
		})
	}
	out := map[string]interface{}{
		"seen":     st.Seen,
		"articles": articles,
	}
	data, _ := json.MarshalIndent(out, "", "  ")
	ioutil.WriteFile(path, data, 0644)

	// CSV export
	os.MkdirAll("export", 0755)
	f, err := os.Create("export/export.csv")
	if err == nil {
		defer f.Close()
		f.WriteString("title,url,fetched,tags,sentiment,summary,thread_url\n")
		for _, it := range items {
			line := fmt.Sprintf(
				"\"%s\",%s,%s,\"%s\",%s,\"%s\",%s\n",
				strings.ReplaceAll(it.Title, "\"", "'"),
				it.URL,
				it.Fetched.Format(time.RFC3339),
				strings.Join(it.Tags, ";"),
				it.Sentiment,
				strings.ReplaceAll(it.Summary, "\"", "'"),
				it.ThreadURL,
			)
			f.WriteString(line)
		}
	}
}

func updateState(st *State, items []Item) {
	for _, it := range items {
		st.Seen[it.Hash] = it.Fetched
	}
}

func fetchAll(cfg *Config, client *http.Client) []Item {
	var all []Item
	for _, src := range cfg.RSS {
		all = append(all, fetchRSS(src, client)...)
	}
	for _, src := range cfg.HTML {
		all = append(all, fetchHTML(src, client)...)
	}
	return all
}

func fetchRSS(src struct{ Name, URL string; Tags []string }, client *http.Client) []Item {
	resp, err := client.Get(src.URL)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	doc, _ := goquery.NewDocumentFromReader(resp.Body)
	var out []Item
	doc.Find("item").Each(func(_ int, s *goquery.Selection) {
		title := s.Find("title").Text()
		link := s.Find("link").Text()
		desc := s.Find("description").Text()
		hash := fmt.Sprintf("%x", md5.Sum([]byte(title+link)))
		out = append(out, Item{Hash: hash, Title: title, URL: link, Tags: src.Tags, Fetched: time.Now(), Body: desc})
	})
	return out
}

func fetchHTML(src struct{ Name, URL, Selector string; Tags []string }, client *http.Client) []Item {
	resp, err := client.Get(src.URL)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	doc, _ := goquery.NewDocumentFromReader(resp.Body)
	var out []Item
	doc.Find(src.Selector).Each(func(_ int, s *goquery.Selection) {
		title := s.Text()
		link, _ := s.Find("a").Attr("href")
		body := s.Text()
		hash := fmt.Sprintf("%x", md5.Sum([]byte(title+link)))
		out = append(out, Item{Hash: hash, Title: title, URL: link, Tags: src.Tags, Fetched: time.Now(), Body: body})
	})
	return out
}

func filterNew(items []Item, st *State) []Item {
	var out []Item
	for _, it := range items {
		if _, seen := st.Seen[it.Hash]; !seen {
			out = append(out, it)
		}
	}
	return out
}

func pickTop(items []Item, max int) []Item {
	sort.Slice(items, func(i, j int) bool {
		return items[i].Fetched.After(items[j].Fetched)
	})
	if len(items) > max {
		return items[:max]
	}
	return items
}

func filterKeywords(items []Item, keywords []string) []Item {
	var out []Item
	for _, it := range items {
		text := strings.ToLower(it.Title + " " + it.Body)
		for _, kw := range keywords {
			if strings.Contains(text, strings.ToLower(kw)) {
				out = append(out, it)
				break
			}
		}
	}
	return out
}

func classify(items []Item, client *openai.Client) {
	ctx := context.Background()
	for i := range items {
		// topic tag
		topic := fmt.Sprintf("Tag a single topic keyword for this article:\n%s", items[i].Title)
		resp1, _ := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model:    "gpt-3.5-turbo",
			Messages: []openai.ChatCompletionMessage{{Role: "user", Content: topic}},
			MaxTokens:  10,
		})
		items[i].Tags = append(items[i].Tags, strings.TrimSpace(resp1.Choices[0].Message.Content))

		// sentiment
		sent := fmt.Sprintf("Classify sentiment (Positive/Negative/Neutral):\n%s", items[i].Body)
		resp2, _ := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model:    "gpt-3.5-turbo",
			Messages: []openai.ChatCompletionMessage{{Role: "user", Content: sent}},
			MaxTokens:  10,
		})
		items[i].Sentiment = strings.TrimSpace(resp2.Choices[0].Message.Content)
	}
}

func batchSummarize(items []Item, client *openai.Client) []string {
	ctx := context.Background()
	var bodies []string
	for _, it := range items {
		bodies = append(bodies, it.Body)
	}
	prompt := fmt.Sprintf("Summarize each article into three paragraphs, separated by '---':\n%s", strings.Join(bodies, "\n---\n"))
	resp, _ := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    "gpt-3.5-turbo",
		Messages: []openai.ChatCompletionMessage{{Role: "user", Content: prompt}},
		MaxTokens: 3600,
	})
	parts := strings.Split(resp.Choices[0].Message.Content, "---")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func buildEmbed(trending, others, watch []Item, cfg *Config) string {
	// load alerts
	alertTags := make(map[string]bool)
	alertMention := os.Getenv("ALERT_MENTION")
	if data, err := ioutil.ReadFile("config/alert_tags.json"); err == nil {
		var conf struct {
			AlertTags   []string `json:"alert_tags"`
			AlertTarget string   `json:"alert_target"`
		}
		json.Unmarshal(data, &conf)
		for _, t := range conf.AlertTags {
			alertTags[strings.ToLower(t)] = true
		}
		if conf.AlertTarget != "" {
			alertMention = conf.AlertTarget
		}
	}

	embed := map[string]interface{}{
		"embeds": []map[string]interface{}{{
			"title": "📰 Sankarea Digest",
			"color": cfg.Colors.Default,
			"fields": []map[string]string{},
		}},
	}
	fields := []map[string]string{}

	for _, it := range trending {
		alertLine := ""
		for _, tag := range it.Tags {
			if alertTags[strings.ToLower(tag)] {
				alertLine = "\n🚨 " + alertMention
				break
			}
		}
		fields = append(fields, map[string]string{
			"name":  "🔥 " + it.Title,
			"value": it.Summary + "\n" + it.URL + alertLine,
		})
	}
	for _, it := range others {
		fields = append(fields, map[string]string{
			"name":  it.Title,
			"value": it.URL,
		})
	}
	if len(watch) > 0 {
		var names []string
		for _, it := range watch {
			names = append(names, it.Title)
		}
		fields = append(fields, map[string]string{
			"name":  "⚠️ Watchlist",
			"value": strings.Join(names, ", "),
		})
	}

	embedArr := embed["embeds"].([]map[string]interface{})
	embedArr[0]["fields"] = fields
	b, _ := json.Marshal(embed)
	return string(b)
}

func postDiscord(embed string) {
	channelID := os.Getenv("DISCORD_CHANNEL_ID")
	token := os.Getenv("DISCORD_BOT_TOKEN")
	if channelID == "" || token == "" {
		log.Println("Missing channel ID or bot token")
		return
	}

	// create thread
	threadReq := map[string]interface{}{
		"name":                  fmt.Sprintf("Digest %s", time.Now().Format("01/02 15:04")),
		"auto_archive_duration": 1440,
		"type":                  11,
	}
	bodyBytes, _ := json.Marshal(threadReq)
	threadURL := fmt.Sprintf("https://discord.com/api/v10/channels/%s/threads", channelID)
	req, _ := http.NewRequest("POST", threadURL, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Authorization", "Bot "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("Thread creation error:", err)
		return
	}
	defer resp.Body.Close()

	var result struct{ ID string `json:"id"` }
	json.NewDecoder(resp.Body).Decode(&result)

	// post embed
	msgURL := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages", result.ID)
	req2, _ := http.NewRequest("POST", msgURL, strings.NewReader(embed))
	req2.Header.Set("Authorization", "Bot "+token)
	req2.Header.Set("Content-Type", "application/json")
	http.DefaultClient.Do(req2)
}

func archiveStaleThreads() {
	token := os.Getenv("DISCORD_BOT_TOKEN")
	if token == "" {
		return
	}
	data, err := ioutil.ReadFile("data/state.json")
	if err != nil {
		return
	}
	var state struct {
		Articles []struct {
			ThreadURL string    `json:"thread_url"`
			Fetched   time.Time `json:"fetched"`
		} `json:"articles"`
	}
	json.Unmarshal(data, &state)

	for _, a := range state.Articles {
		if a.ThreadURL == "" || time.Since(a.Fetched) < 48*time.Hour {
			continue
		}
		parts := strings.Split(a.ThreadURL, "/")
		threadID := parts[len(parts)-1]
		url := fmt.Sprintf("https://discord.com/api/v10/channels/%s", threadID)
		payload := strings.NewReader(`{"archived":true}`)
		req, _ := http.NewRequest("PATCH", url, payload)
		req.Header.Set("Authorization", "Bot "+token)
		req.Header.Set("Content-Type", "application/json")
		http.DefaultClient.Do(req)
	}
}

// admin helpers
func getAlertTags() []string {
	data, _ := ioutil.ReadFile("config/alert_tags.json")
	var conf struct{ AlertTags []string `json:"alert_tags"` }
	json.Unmarshal(data, &conf)
	return conf.AlertTags
}

func modifyAlertTags(cmd, tag string) {
	data, _ := ioutil.ReadFile("config/alert_tags.json")
	var conf struct {
		AlertTags   []string `json:"alert_tags"`
		AlertTarget string   `json:"alert_target"`
	}
	json.Unmarshal(data, &conf)
	switch cmd {
	case "addtag":
		conf.AlertTags = appendIfMissing(conf.AlertTags, tag)
	case "removetag":
		conf.AlertTags = removeTag(conf.AlertTags, tag)
	}
	out, _ := json.MarshalIndent(conf, "", "  ")
	ioutil.WriteFile("config/alert_tags.json", out, 0644)
}

func appendIfMissing(slice []string, val string) []string {
	for _, item := range slice {
		if strings.EqualFold(item, val) {
			return slice
		}
	}
	return append(slice, val)
}

func removeTag(slice []string, val string) []string {
	var out []string
	for _, item := range slice {
		if !strings.EqualFold(item, val) {
			out = append(out, item)
		}
	}
	return out
}
