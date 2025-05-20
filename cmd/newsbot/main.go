package main

import (
  "context"
  "crypto/md5"
  "encoding/json"
  "flag"
  "fmt"
  "io/ioutil"
  "log"
  "net/http"
  "net/url"
  "os"
  "path/filepath"
  "sort"
  "strings"
  "time"

  "github.com/PuerkitoBio/goquery"
  openai "github.com/sashabaranov/go-openai"
  "gopkg.in/yaml.v3"
)

type Config struct {
  Keywords []string
  FeedTypes map[string]struct {
    TTLDays int
    SimilarityThreshold float64
  } `yaml:"feed_types"`
  Colors struct {
    Default int
    Government int
  }
  RSS []struct {
    Name string
    URL string
    Tags []string
  }
  HTML []struct {
    Name string
    URL string
    Selector string
    Tags []string
  }
}

type Item struct {
  Hash string
  Title string
  URL string
  Tags []string
  Summary string
  Sentiment string
  Fetched time.Time
  Body string
  ThreadURL string
}

type State struct {
  Seen map[string]time.Time `json:"seen"`
}

func main() {
  configPath := flag.String("config", "config/sources.yml", "")
  statePath := flag.String("state", "data/state.json", "")
  interval := flag.Int("interval", 24, "")
  maxItems := flag.Int("max", 6, "")
  flag.Parse()

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

    updateState(st, items)
    saveState(*statePath, st, items)

    // Post to Discord and open thread if desired (not implemented here)
    break // or sleep + repeat
  }
}

// Load/save config/state
func loadConfig(path string) *Config {
  data, _ := ioutil.ReadFile(path)
  var cfg Config
  yaml.Unmarshal(data, &cfg)
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
  var articles []map[string]interface{}
  for _, it := range items {
    articles = append(articles, map[string]interface{}{
      "hash": it.Hash, "title": it.Title, "url": it.URL,
      "tags": it.Tags, "summary": it.Summary,
      "sentiment": it.Sentiment, "fetched": it.Fetched,
      "thread_url": it.ThreadURL,
    })
  }
  out := map[string]interface{}{
    "seen": st.Seen,
    "articles": articles,
  }
  data, _ := json.MarshalIndent(out, "", "  ")
  ioutil.WriteFile(path, data, 0644)
}

// Fetching
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

func fetchRSS(src struct{Name, URL string; Tags []string}, client *http.Client) []Item {
  resp, _ := client.Get(src.URL)
  defer resp.Body.Close()
  doc, _ := goquery.NewDocumentFromReader(resp.Body)
  var out []Item
  doc.Find("item").Each(func(i int, s *goquery.Selection) {
    title := s.Find("title").Text()
    link := s.Find("link").Text()
    desc := s.Find("description").Text()
    hash := fmt.Sprintf("%x", md5.Sum([]byte(title+link)))
    out = append(out, Item{
      Hash: hash, Title: title, URL: link, Tags: src.Tags,
      Fetched: time.Now(), Body: desc,
    })
  })
  return out
}

func fetchHTML(src struct{Name, URL, Selector string; Tags []string}, client *http.Client) []Item {
  resp, _ := client.Get(src.URL)
  defer resp.Body.Close()
  doc, _ := goquery.NewDocumentFromReader(resp.Body)
  var out []Item
  doc.Find(src.Selector).Each(func(i int, s *goquery.Selection) {
    title := s.Text()
    link, _ := s.Find("a").Attr("href")
    body := s.Text()
    hash := fmt.Sprintf("%x", md5.Sum([]byte(title+link)))
    out = append(out, Item{
      Hash: hash, Title: title, URL: link, Tags: src.Tags,
      Fetched: time.Now(), Body: body,
    })
  })
  return out
}

// Logic
func filterNew(items []Item, st *State) []Item {
  var out []Item
  for _, it := range items {
    if _, ok := st.Seen[it.Hash]; !ok {
      out = append(out, it)
    }
  }
  return out
}

func classify(items []Item, client *openai.Client) {
  for i := range items {
    prompt := fmt.Sprintf("Topic: %s", items[i].Title)
    resp, _ := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
      Model: "gpt-3.5-turbo",
      Messages: []openai.ChatCompletionMessage{{Role: "user", Content: prompt}},
      MaxTokens: 10,
    })
    items[i].Tags = append(items[i].Tags, strings.TrimSpace(resp.Choices[0].Message.Content))

    sentPrompt := fmt.Sprintf("Sentiment: %s", items[i].Body)
    resp, _ = client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
      Model: "gpt-3.5-turbo",
      Messages: []openai.ChatCompletionMessage{{Role: "user", Content: sentPrompt}},
      MaxTokens: 10,
    })
    items[i].Sentiment = strings.TrimSpace(resp.Choices[0].Message.Content)
  }
}

func batchSummarize(items []Item, client *openai.Client) []string {
  var bodies []string
  for _, it := range items {
    bodies = append(bodies, it.Body)
  }
  prompt := "Summarize:\n" + strings.Join(bodies, "\n---\n")
  resp, _ := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
    Model: "gpt-3.5-turbo",
    Messages: []openai.ChatCompletionMessage{{Role: "user", Content: prompt}},
    MaxTokens: 3600,
  })
  return strings.Split(resp.Choices[0].Message.Content, "---")
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

func updateState(st *State, items []Item) {
  for _, it := range items {
    st.Seen[it.Hash] = it.Fetched
  }
}
