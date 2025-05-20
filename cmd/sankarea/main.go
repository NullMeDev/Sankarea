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
    "sort"
    "strings"
    "time"

    "github.com/PuerkitoBio/goquery"
    openai "github.com/sashabaranov/go-openai"
    "gopkg.in/yaml.v3"
)

type Config struct {
    RSS  []Source `yaml:"rss"`
    HTML []Source `yaml:"html"`
}

type Source struct {
    Name     string   `yaml:"name"`
    URL      string   `yaml:"url"`
    Selector string   `yaml:"selector,omitempty"`
    Tags     []string `yaml:"tags"`
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
    cfgPath := flag.String("config", "config/sources.yml", "")
    stPath := flag.String("state", "data/state.json", "")
    flag.Parse()

    log.SetFlags(log.LstdFlags | log.Lshortfile)
    cfg := loadConfig(*cfgPath)
    st := loadState(*stPath)
    client := openai.NewClient(os.Getenv("OPENAI_API_KEY"))
    httpc := &http.Client{Timeout: 15 * time.Second}

    items := fetchAll(cfg, httpc)
    newItems := filterNew(items, st)
    if len(newItems) == 0 {
        log.Println("✔ no new items")
        saveState(*stPath, st)
        return
    }

    sort.Slice(newItems, func(i, j int) bool {
        return newItems[i].Fetched.After(newItems[j].Fetched)
    })
    if len(newItems) > 6 {
        newItems = newItems[:6]
    }

    var bodies []string
    for _, it := range newItems {
        bodies = append(bodies, it.Body)
    }
    prompt := fmt.Sprintf(
        "Summarize each of these %d articles into a 3-paragraph summary, separated by '---':\n%s",
        len(bodies), strings.Join(bodies, "\n---\n"),
    )
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
    for i := range newItems {
        newItems[i].Summary = strings.TrimSpace(parts[i])
    }

    var builder strings.Builder
    builder.WriteString("📰 **Sankarea Digest**\n\n")
    for _, it := range newItems {
        builder.WriteString(fmt.Sprintf(
            "🔥 **%s**\n%s\n<%s>\n\n",
            it.Title, it.Summary, it.URL,
        ))
    }
    payload := map[string]string{"content": builder.String()}
    data, _ := json.Marshal(payload)
    log.Printf("▶ payload: %s\n", data)

    for _, hook := range strings.Split(os.Getenv("DISCORD_WEBHOOKS"), ",") {
        res, err := http.Post(hook, "application/json", bytes.NewReader(data))
        if err != nil {
            log.Printf("✖ post error: %v", err)
        } else {
            log.Printf("✓ posted to %s status=%s", hook, res.Status)
            res.Body.Close()
        }
    }

    for _, it := range newItems {
        st.Seen[it.Hash] = it.Fetched
    }
    saveState(*stPath, st)
}

// … helper functions (loadConfig, loadState, saveState, fetchAll, fetchRSS, fetchHTML, filterNew) unchanged …
