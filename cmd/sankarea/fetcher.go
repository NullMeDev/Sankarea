// cmd/sankarea/fetcher.go
package main

import (
    "context"
    "crypto/md5"
    "fmt"
    "io"
    "net/http"
    "strings"
    "sync"
    "time"

    "github.com/mmcdole/gofeed"
)

// Fetcher handles retrieving news from RSS feeds
type Fetcher struct {
    client     *http.Client
    parser     *gofeed.Parser
    lastFetch  map[string]time.Time
    fetchMutex sync.RWMutex
}

// FetchResult represents the result of a fetch operation
type FetchResult struct {
    Source     string
    Articles   []*NewsArticle
    Error      error
    FetchTime  time.Time
}

// NewFetcher creates a new news fetcher instance
func NewFetcher() *Fetcher {
    return &Fetcher{
        client: &http.Client{
            Timeout: 30 * time.Second,
        },
        parser:    gofeed.NewParser(),
        lastFetch: make(map[string]time.Time),
    }
}

// FetchAll retrieves news from all active sources
func (f *Fetcher) FetchAll(ctx context.Context) ([]*NewsArticle, error) {
    sources, err := LoadSources()
    if err != nil {
        return nil, fmt.Errorf("failed to load sources: %v", err)
    }

    // Create channel for results
    results := make(chan FetchResult, len(sources))
    var wg sync.WaitGroup

    // Fetch from each source concurrently
    for _, source := range sources {
        // Skip paused sources
        if source.Paused {
            continue
        }

        wg.Add(1)
        go func(s NewsSource) {
            defer wg.Done()
            articles, err := f.FetchSource(ctx, s)
            results <- FetchResult{
                Source:    s.Name,
                Articles:  articles,
                Error:     err,
                FetchTime: time.Now(),
            }
        }(source)
    }

    // Wait for all fetches to complete in a separate goroutine
    go func() {
        wg.Wait()
        close(results)
    }()

    // Collect results
    var allArticles []*NewsArticle
    var errors []string

    for result := range results {
        if result.Error != nil {
            errors = append(errors, fmt.Sprintf("%s: %v", result.Source, result.Error))
            continue
        }

        // Update last fetch time
        f.fetchMutex.Lock()
        f.lastFetch[result.Source] = result.FetchTime
        f.fetchMutex.Unlock()

        allArticles = append(allArticles, result.Articles...)
    }

    // Log any errors that occurred
    if len(errors) > 0 {
        Logger().Printf("Errors during fetch: %s", strings.Join(errors, "; "))
    }

    return allArticles, nil
}

// FetchSource retrieves news from a single source
func (f *Fetcher) FetchSource(ctx context.Context, source NewsSource) ([]*NewsArticle, error) {
    // Check if we should throttle requests
    if !f.shouldFetch(source.Name) {
        return nil, nil
    }

    // Fetch feed
    feed, err := f.fetchFeed(ctx, source.URL)
    if err != nil {
        return nil, err
    }

    // Convert feed items to articles
    var articles []*NewsArticle
    for _, item := range feed.Items {
        // Skip items without required fields
        if item.Title == "" || item.Link == "" {
            continue
        }

        // Parse publication date
        pubDate := item.PublishedParsed
        if pubDate == nil {
            pubDate = &time.Time{}
        }

        // Create article
        article := &NewsArticle{
            ID:          generateArticleID(item.Link),
            Title:       item.Title,
            Content:     getArticleContent(item),
            URL:        item.Link,
            Source:     source.Name,
            Category:   source.Category,
            PublishedAt: *pubDate,
            FetchedAt:  time.Now(),
            Citations:  extractCitations(item),
        }

        // Perform fact checking if enabled for this source
        if source.FactCheck {
            fc := NewFactChecker()
            result, err := fc.CheckArticle(ctx, article)
            if err != nil {
                Logger().Printf("Warning: fact check failed for %s: %v", article.URL, err)
            } else {
                article.FactCheckResult = result
            }
        }

        articles = append(articles, article)
    }

    return articles, nil
}

// shouldFetch checks if enough time has passed since the last fetch
func (f *Fetcher) shouldFetch(source string) bool {
    f.fetchMutex.RLock()
    lastFetch, exists := f.lastFetch[source]
    f.fetchMutex.RUnlock()

    if !exists {
        return true
    }

    return time.Since(lastFetch) >= time.Duration(cfg.NewsIntervalMinutes)*time.Minute
}

// fetchFeed retrieves and parses an RSS feed
func (f *Fetcher) fetchFeed(ctx context.Context, url string) (*gofeed.Feed, error) {
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, err
    }

    resp, err := f.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
    }

    return f.parser.Parse(resp.Body)
}

// Helper functions

func getArticleContent(item *gofeed.Item) string {
    // Try different fields for content
    if item.Content != "" {
        return item.Content
    }
    if item.Description != "" {
        return item.Description
    }
    return item.Title
}

func extractCitations(item *gofeed.Item) []string {
    var citations []string

    // Extract links from content
    if item.Content != "" {
        citations = append(citations, extractURLs(item.Content)...)
    }

    // Add enclosures
    for _, enclosure := range item.Enclosures {
        if enclosure.URL != "" {
            citations = append(citations, enclosure.URL)
        }
    }

    return unique(citations)
}

func extractURLs(content string) []string {
    var urls []string
    // Simple URL extraction - could be improved with regex
    words := strings.Fields(content)
    for _, word := range words {
        if strings.HasPrefix(word, "http://") || strings.HasPrefix(word, "https://") {
            urls = append(urls, word)
        }
    }
    return urls
}

func generateArticleID(url string) string {
    // Create a unique ID based on URL and timestamp
    return fmt.Sprintf("%x", md5.Sum([]byte(url+time.Now().String())))
}

func unique(slice []string) []string {
    keys := make(map[string]bool)
    var list []string
    for _, entry := range slice {
        if _, value := keys[entry]; !value {
            keys[entry] = true
            list = append(list, entry)
        }
    }
    return list
}
