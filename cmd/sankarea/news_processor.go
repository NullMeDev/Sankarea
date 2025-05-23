// cmd/sankarea/news_processor.go
package main

import (
    "context"
    "fmt"
    "html"
    "io"
    "net/http"
    "strings"
    "sync"
    "time"

    "github.com/mmcdole/gofeed"
)

// NewsProcessor handles the fetching and processing of RSS feeds
type NewsProcessor struct {
    parser      *gofeed.Parser
    client      *http.Client
    cache       map[string]time.Time
    cacheMutex  sync.RWMutex
    maxArticles int
}

// NewNewsProcessor creates a new NewsProcessor instance
func NewNewsProcessor() *NewsProcessor {
    return &NewsProcessor{
        parser: gofeed.NewParser(),
        client: &http.Client{
            Timeout: 30 * time.Second,
        },
        cache:       make(map[string]time.Time),
        maxArticles: 5, // Default max articles per source
    }
}

// ProcessFeeds fetches and processes all enabled feeds
func (np *NewsProcessor) ProcessFeeds(ctx context.Context, sources []Source) ([]*NewsArticle, error) {
    var (
        articles = make([]*NewsArticle, 0)
        wg       sync.WaitGroup
        mu       sync.Mutex
        results  = make(chan *NewsArticle, len(sources)*np.maxArticles)
        errors   = make(chan error, len(sources))
    )

    // Process each source concurrently
    for _, source := range sources {
        wg.Add(1)
        go func(src Source) {
            defer wg.Done()
            
            // Fetch and process the feed
            feedArticles, err := np.processFeed(ctx, src)
            if err != nil {
                errors <- fmt.Errorf("error processing %s: %v", src.Name, err)
                return
            }

            // Send articles to results channel
            for _, article := range feedArticles {
                select {
                case results <- article:
                case <-ctx.Done():
                    return
                }
            }
        }(source)
    }

    // Wait for all goroutines to complete in a separate goroutine
    go func() {
        wg.Wait()
        close(results)
        close(errors)
    }()

    // Collect results and errors
    var errs []string
    for err := range errors {
        errs = append(errs, err.Error())
    }

    // Collect articles
    for article := range results {
        articles = append(articles, article)
    }

    // Check for errors
    if len(errs) > 0 {
        return articles, fmt.Errorf("feed processing errors: %s", strings.Join(errs, "; "))
    }

    return articles, nil
}

// processFeed fetches and processes a single feed
func (np *NewsProcessor) processFeed(ctx context.Context, source Source) ([]*NewsArticle, error) {
    // Check context cancellation
    if ctx.Err() != nil {
        return nil, ctx.Err()
    }

    // Create request with context
    req, err := http.NewRequestWithContext(ctx, "GET", source.URL, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %v", err)
    }

    // Set user agent
    req.Header.Set("User-Agent", "Sankarea News Bot/1.0")

    // Fetch feed
    resp, err := np.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("failed to fetch feed: %v", err)
    }
    defer resp.Body.Close()

    // Read response body
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read response: %v", err)
    }

    // Parse feed
    feed, err := np.parser.ParseString(string(body))
    if err != nil {
        return nil, fmt.Errorf("failed to parse feed: %v", err)
    }

    // Process articles
    var articles []*NewsArticle
    for _, item := range feed.Items {
        // Skip if we have enough articles
        if len(articles) >= np.maxArticles {
            break
        }

        // Skip if article is already processed
        if np.isArticleProcessed(item.GUID) {
            continue
        }

        // Create article
        article := &NewsArticle{
            Title:       html.UnescapeString(item.Title),
            URL:         item.Link,
            Description: np.processDescription(item.Description),
            Category:    source.Category,
            SourceName:  source.Name,
            PublishedAt: np.getPublishDate(item),
        }

        // Extract image if available
        if item.Image != nil {
            article.ImageURL = item.Image.URL
        } else if enclosure := np.findImageEnclosure(item); enclosure != nil {
            article.ImageURL = enclosure.URL
        }

        articles = append(articles, article)
        np.markArticleProcessed(item.GUID)
    }

    return articles, nil
}

// isArticleProcessed checks if an article has been processed
func (np *NewsProcessor) isArticleProcessed(guid string) bool {
    np.cacheMutex.RLock()
    defer np.cacheMutex.RUnlock()
    
    processedTime, exists := np.cache[guid]
    if !exists {
        return false
    }
    
    // Remove from cache if older than 24 hours
    if time.Since(processedTime) > 24*time.Hour {
        delete(np.cache, guid)
        return false
    }
    
    return true
}

// markArticleProcessed marks an article as processed
func (np *NewsProcessor) markArticleProcessed(guid string) {
    np.cacheMutex.Lock()
    defer np.cacheMutex.Unlock()
    np.cache[guid] = time.Now()
}

// processDescription cleans and truncates the description
func (np *NewsProcessor) processDescription(desc string) string {
    // Unescape HTML entities
    desc = html.UnescapeString(desc)
    
    // Remove HTML tags
    desc = stripHTML(desc)
    
    // Truncate if too long (Discord limit is 2048 characters)
    if len(desc) > 2000 {
        desc = desc[:1997] + "..."
    }
    
    return strings.TrimSpace(desc)
}

// getPublishDate gets the publish date of an article
func (np *NewsProcessor) getPublishDate(item *gofeed.Item) time.Time {
    if item.PublishedParsed != nil {
        return *item.PublishedParsed
    }
    if item.UpdatedParsed != nil {
        return *item.UpdatedParsed
    }
    return time.Now()
}

// findImageEnclosure finds an image enclosure in the feed item
func (np *NewsProcessor) findImageEnclosure(item *gofeed.Item) *gofeed.Enclosure {
    for _, enc := range item.Enclosures {
        if strings.HasPrefix(enc.Type, "image/") {
            return enc
        }
    }
    return nil
}

// stripHTML removes HTML tags from a string
func stripHTML(input string) string {
    var output strings.Builder
    var inTag bool
    
    for _, r := range input {
        switch {
        case r == '<':
            inTag = true
        case r == '>':
            inTag = false
        case !inTag:
            output.WriteRune(r)
        }
    }
    
    return output.String()
}
