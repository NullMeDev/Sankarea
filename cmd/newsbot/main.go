package main

import (
  "context"
  "encoding/json"
  "flag"
  "fmt"
  "io/ioutil"
  "log"
  "net/http"
  "os"
  "time"

  "github.com/PuerkitoBio/goquery"
  openai "github.com/sashabaranov/go-openai"
  "gopkg.in/yaml.v3"
)

type Config struct {
  RSS []struct {
    Name     string   `yaml:"name"`
    URL      string   `yaml:"url"`
    Tags     []string `yaml:"tags"`
  } `yaml:"rss"`
  HTML []struct {
    Name     string   `yaml:"name"`
    URL      string   `yaml:"url"`
    Selector string   `yaml:"selector"`
    Tags     []string `yaml:"tags"`
  } `yaml:"html"`
}

type Item struct {
  Hash    string
  Title   string
  URL     string
  Body    string
  Tags    []string
  Summary string
  Fetched time.Time
}

type State struct {
  Seen map[string]time.Time `json:"seen"`
}

func main() {
  cfgPath := flag.String("config", "config/sources.yml", "YAML config")
  stPath  := flag.String("state",  "data/state.json",   "JSON state")
  flag.Parse()

  cfg := loadConfig(*cfgPath)
  st  := loadState(*stPath)
  client := openai.NewClient(os.Getenv("OPENAI_API_KEY"))
  httpc  := &http.Client{Timeout: 15 * time.Second}

  // Single iteration (GitHub Action will exit)
  items := fetchAll(cfg, httpc)
  newItems := filterNew(items, st)
  if len(newItems) == 0 {
    log.Println("No new items; nothing to post.")
  } else {
    top := pickTop(newItems, 6)
    bodies := []string{}
    for _, it := range top {
      bodies = append(bodies, it.Body)
    }
    // Batch summarize
    prompt := fmt.Sprintf("Summarize %d articles into three paragraphs each separated by '---':\n%s",
      len(bodies),    strings.Join(bodies, "\n---\n"))
    resp, err := client.CreateChatCompletion(
      context.Background(),
      openai.ChatCompletionRequest{
        Model:    "gpt-3.5-turbo",
        Messages: []openai.ChatCompletionMessage{{Role: "user", Content: prompt}},
        MaxTokens: 2000,
      },
    )
    if err != nil {
      log.Fatalf("OpenAI error: %v", err)
    }
    parts := strings.Split(resp.Choices[0].Message.Content, "---")
    for i := range top {
      top[i].Summary = strings.TrimSpace(parts[i])
    }

    // Build a simple text payload
    payload := map[string]interface{}{
      "content": "📰 **Sankarea Digest**\n\n" +
        strings.Join(func() []string {
          lines := []string{}
          for _, it := range top {
            lines = append(lines,
              fmt.Sprintf("🔥 **%s**\n%s\n<%s>\n", it.Title, it.Summary, it.URL))
          }
          return lines
        }(), "\n"),
    }
    data, _ := json.Marshal(payload)
    log.Printf("Posting payload: %s\n", data) // ← DEBUG LOG

    // Post to all webhooks
    for _, hook := range strings.Split(os.Getenv("DISCORD_WEBHOOKS"), ",") {
      resp, err := http.Post(hook, "application/json", bytes.NewReader(data))
      if err != nil {
        log.Printf("Post error: %v", err)
      } else {
        log.Printf("Posted to %s → %s", hook, resp.Status)
        resp.Body.Close()
      }
    }
  }

  // Update state & save
  for _, it := range newItems {
    st.Seen[it.Hash] = it.Fetched
  }
  saveState(*stPath, st)
}

// (Implement loadConfig, loadState, saveState, fetchAll, fetchRSS, fetchHTML, filterNew, pickTop…)
