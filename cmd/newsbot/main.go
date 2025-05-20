package main

import ( "bytes" "context" "crypto/md5" "encoding/json" "flag" "fmt" "io/ioutil" "log" "net/http" "net/url" "os" "path/filepath" "sort" "strings" "time"

"github.com/PuerkitoBio/goquery"
openai "github.com/sashabaranov/go-openai"
"gopkg.in/yaml.v3"

)

type Config struct { Keywords  []string FeedTypes map[string]struct { TTLDays            int     yaml:"ttl_days" SimilarityThreshold float64 yaml:"similarity_threshold" } yaml:"feed_types" Colors struct { Default    int yaml:"default" Government int yaml:"government" } RSS []struct { Name string   yaml:"name" URL  string   yaml:"url" Tags []string yaml:"tags" } yaml:"rss" HTML []struct { Name     string   yaml:"name" URL      string   yaml:"url" Selector string   yaml:"selector" Tags     []string yaml:"tags" } yaml:"html" }

type Item struct { Hash      string Title     string URL       string Tags      []string Summary   string Sentiment string Fetched   time.Time Body      string ThreadURL string }

type State struct { Seen map[string]time.Time json:"seen" }

func main() { configPath := flag.String("config", "config/sources.yml", "") statePath := flag.String("state", "data/state.json", "") interval := flag.Int("interval", 24, "") maxItems := flag.Int("max", 6, "") flag.Parse()

cfg := loadConfig(*configPath)
st := loadState(*statePath)
client := openai.NewClient(os.Getenv("OPENAI_API_KEY"))
httpClient := &http.Client{Timeout: 20 * time.Second}

for {
	items := fetchAll(cfg, httpClient)
	items = filterNew(items, st)
	classify(items, client)

	trending := pickTop(items, *maxItems)
	summaries := batchSummarize(trending, client)
	for i := range trending {
		trending[i].Summary = summaries[i]
	}

	watch := filterKeywords(items, cfg.Keywords)
	embed := buildEmbed(trending, items, watch, cfg)
	postDiscord(embed)

	updateState(st, items)
	saveState(*statePath, st, items)

	break // remove to loop every interval
}

}

func loadConfig(path string) *Config { data, _ := ioutil.ReadFile(path) var cfg Config yaml.Unmarshal(data, &cfg) return &cfg }

func loadState(path string) *State { st := &State{Seen: make(map[string]time.Time)} data, _ := ioutil.ReadFile(path) json.Unmarshal(data, st) return st }

func saveState(path string, st *State, items []Item) { os.MkdirAll(filepath.Dir(path), 0755) var articles []map[string]interface{} for _, it := range items { articles = append(articles, map[string]interface{}{ "hash": it.Hash, "title": it.Title, "url": it.URL, "tags": it.Tags, "summary": it.Summary, "sentiment": it.Sentiment, "fetched": it.Fetched, "thread_url": it.ThreadURL, }) } out := map[string]interface{}{ "seen": st.Seen, "articles": articles, } data, _ := json.MarshalIndent(out, "", "  ") ioutil.WriteFile(path, data, 0644) }

func updateState(st *State, items []Item) { for _, it := range items { st.Seen[it.Hash] = it.Fetched } }

func fetchAll(cfg *Config, client *http.Client) []Item { var all []Item for _, src := range cfg.RSS { all = append(all, fetchRSS(src, client)...) } for _, src := range cfg.HTML { all = append(all, fetchHTML(src, client)...) } return all }

func fetchRSS(src struct{Name, URL string; Tags []string}, client *http.Client) []Item { resp, _ := client.Get(src.URL) defer resp.Body.Close() doc, _ := goquery.NewDocumentFromReader(resp.Body) var out []Item doc.Find("item").Each(func(i int, s *goquery.Selection) { title := s.Find("title").Text() link := s.Find("link").Text() desc := s.Find("description").Text() hash := fmt.Sprintf("%x", md5.Sum([]byte(title+link))) out = append(out, Item{ Hash: hash, Title: title, URL: link, Tags: src.Tags, Fetched: time.Now(), Body: desc, }) }) return out }

func fetchHTML(src struct{Name, URL, Selector string; Tags []string}, client *http.Client) []Item { resp, _ := client.Get(src.URL) defer resp.Body.Close() doc, _ := goquery.NewDocumentFromReader(resp.Body) var out []Item doc.Find(src.Selector).Each(func(i int, s *goquery.Selection) { title := s.Text() link, _ := s.Find("a").Attr("href") body := s.Text() hash := fmt.Sprintf("%x", md5.Sum([]byte(title+link))) out = append(out, Item{ Hash: hash, Title: title, URL: link, Tags: src.Tags, Fetched: time.Now(), Body: body, }) }) return out }

func filterNew(items []Item, st *State) []Item { var out []Item for _, it := range items { if _, ok := st.Seen[it.Hash]; !ok { out = append(out, it) } } return out }

func pickTop(items []Item, max int) []Item { sort.Slice(items, func(i, j int) bool { return items[i].Fetched.After(items[j].Fetched) }) if len(items) > max { return items[:max] } return items }

func filterKeywords(items []Item, keywords []string) []Item { var out []Item for _, it := range items { for _, kw := range keywords { if strings.Contains(strings.ToLower(it.Title+" "+it.Body), strings.ToLower(kw)) { out = append(out, it) break } } } return out }

func classify(items []Item, client *openai.Client) { for i := range items { prompt := fmt.Sprintf("Topic: %s", items[i].Title) resp, _ := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{ Model: "gpt-3.5-turbo", Messages: []openai.ChatCompletionMessage{{Role: "user", Content: prompt}}, MaxTokens: 10, }) items[i].Tags = append(items[i].Tags, strings.TrimSpace(resp.Choices[0].Message.Content))

sentPrompt := fmt.Sprintf("Sentiment: %s", items[i].Body)
	resp, _ = client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model: "gpt-3.5-turbo",
		Messages: []openai.ChatCompletionMessage{{Role: "user", Content: sentPrompt}},
		MaxTokens: 10,
	})
	items[i].Sentiment = strings.TrimSpace(resp.Choices[0].Message.Content)
}

}

func batchSummarize(items []Item, client *openai.Client) []string { var bodies []string for _, it := range items { bodies = append(bodies, it.Body) } prompt := "Summarize:\n" + strings.Join(bodies, "\n---\n") resp, _ := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{ Model: "gpt-3.5-turbo", Messages: []openai.ChatCompletionMessage{{Role: "user", Content: prompt}}, MaxTokens: 3600, }) return strings.Split(resp.Choices[0].Message.Content, "---") }

func buildEmbed(trending, others, watch []Item, cfg *Config) string { alertTags := map[string]bool{} alertMention := "@nullmedev" if data, err := ioutil.ReadFile("config/alert_tags.json"); err == nil { var conf struct { AlertTags   []string json:"alert_tags" AlertTarget string   json:"alert_target" } json.Unmarshal(data, &conf) for _, t := range conf.AlertTags { alertTags[strings.ToLower(t)] = true } if conf.AlertTarget != "" { alertMention = conf.AlertTarget } }

embed := map[string]interface{}{
	"embeds": []map[string]interface{}{{
		"title": "📰 Sankarea Digest",
		"color": cfg.Colors.Default,
		"fields": []map[string]string{},
	}},
}

fields := []map[string]string{}
for _, it := range trending {
	shouldAlert := false
	for _, tag := range it.Tags {
		if alertTags[strings.ToLower(tag)] {
			shouldAlert = true
			break
		}
	}
	alertLine := ""
	if shouldAlert {
		alertLine = "\n🚨 " + alertMention
	}

	fields = append(fields, map[string]string{
		"name": "🔥 " + it.Title,
		"value": it.Summary + "\n" + it.URL + alertLine,
	})
}

for _, it := range others {
	fields = append(fields, map[string]string{
		"name": it.Title,
		"value": it.URL,
	})
}

if len(watch) > 0 {
	var names []string
	for _, it := range watch {
		names = append(names, it.Title)
	}
	fields = append(fields, map[string]string{
		"name": "⚠️ Watchlist",
		"value": strings.Join(names, ", "),
	})
}

embedArr := embed["embeds"].([]map[string]interface{})
embedArr[0]["fields"] = fields
b, _ := json.Marshal(embed)
return string(b)

}

func postDiscord(payload string) { webhooks := strings.Split(os.Getenv("DISCORD_WEBHOOKS"), ",") for _, hook := range webhooks { http.Post(hook, "application/json", strings.NewReader(payload)) } }

// Add at the bottom of main.go

func init() { go startCommandServer() }

func startCommandServer() { http.HandleFunc("/command", handleCommand) port := os.Getenv("COMMAND_PORT") if port == "" { port = "9010" } log.Println("Sankarea command server listening on port", port) http.ListenAndServe(":"+port, nil) }

func handleCommand(w http.ResponseWriter, r *http.Request) { if r.Method != http.MethodPost { http.Error(w, "Invalid method", http.StatusMethodNotAllowed) return } token := r.Header.Get("Authorization") if token != os.Getenv("COMMAND_TOKEN") { http.Error(w, "Unauthorized", http.StatusUnauthorized) return }

type Cmd struct {
	Command string `json:"command"`
	Value   string `json:"value"`
}
var cmd Cmd
json.NewDecoder(r.Body).Decode(&cmd)

path := "config/alert_tags.json"
var conf struct {
	AlertTags   []string `json:"alert_tags"`
	AlertTarget string   `json:"alert_target"`
}
data, _ := ioutil.ReadFile(path)
json.Unmarshal(data, &conf)

switch cmd.Command {
case "addtag":
	conf.AlertTags = appendIfMissing(conf.AlertTags, cmd.Value)
case "removetag":
	conf.AlertTags = removeTag(conf.AlertTags, cmd.Value)
case "listtags":
	json.NewEncoder(w).Encode(conf.AlertTags)
	return
default:
	http.Error(w, "Invalid command", http.StatusBadRequest)
	return
}

save, _ := json.MarshalIndent(conf, "", "  ")
ioutil.WriteFile(path, save, 0644)
w.Write([]byte("Success"))

}

func appendIfMissing(slice []string, val string) []string { for _, item := range slice { if strings.ToLower(item) == strings.ToLower(val) { return slice } } return append(slice, val) }

func removeTag(slice []string, val string) []string { var out []string for _, item := range slice { if strings.ToLower(item) != strings.ToLower(val) { out = append(out, item) } } return out }

