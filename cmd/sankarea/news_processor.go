// cmd/sankarea/news_processor.go
package main

import (
    "context"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "html"
    "io"
    "net/http"
    "strings"
    "time"

    "github.com/mmcdole/gofeed"
)

// NewsProcessor handles the fetching and processing of RSS feeds
type NewsProcessor struct {
    parser      *gofeed.Parser
    client      *http.Client
    bot         *Bot
    maxArticles int
}

// NewNewsProcessor creates a new NewsProcessor instance
func NewNewsProcessor(bot *Bot) *NewsProcessor {
    return &NewsProcessor{
        parser: gofeed.NewParser(),
        client: &http.Client{
            Timeout: 30 * time.Second,
        },
        bot:         bot,
        maxArticles: 5,
    }
}

// ProcessFeeds fetches and processes all enabled feeds
func (np *NewsProcessor) ProcessFeeds(ctx context.Context, sources []NewsSource) ([]*NewsArticle, error) {
    var (
        articles = make([]*NewsArticle, 0)
        results  = make(chan *NewsArticle, len(sources)*np.maxArticles)
        errors   = make(chan error, len(sources))
    )

    // Create a wait group to track goroutines
    var wg sync.WaitGroup
    wg.Add(len(sources))

    // Process each source concurrently
    for _, source := range sources {
        if source.Paused {
            wg.Done()
            continue
        }

        go func(src NewsSource) {
            defer wg.Done()
            
            // Fetch and process the feed
            feedArticles, err := np.processFeed(ctx, src)
            if err != nil {
                np.bot.logger.Error("Error processing %s: %v", src.Name, err)
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
func (np *NewsProcessor) processFeed(ctx context.Context, source NewsSource) ([]*NewsArticle, error) {
    // Check context cancellation
    if ctx.Err() != nil {
        return nil, ctx.Err()
    }

    // Create request with context
    req, err := http.NewRequestWithContext(ctx, "GET", source.URL, nil)
    if err != nil {
        np.logFeedError(source, err)
        return nil, fmt.Errorf("failed to create request: %v", err)
    }

    // Set user agent
    req.Header.Set("User-Agent", "Sankarea News Bot/1.0")

    // Fetch feed
    resp, err := np.client.Do(req)
    if err != nil {
        np.logFeedError(source, err)
        return nil, fmt.Errorf("failed to fetch feed: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        err := fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
        np.logFeedError(source, err)
        return nil, err
    }

    // Read response body
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        np.logFeedError(source, err)
        return nil, fmt.Errorf("failed to read response: %v", err)
    }

    // Parse feed
    feed, err := np.parser.ParseString(string(body))
    if err != nil {
        np.logFeedError(source, err)
        return nil, fmt.Errorf("failed to parse feed: %v", err)
    }

    // Process articles
    var articles []*NewsArticle
    for _, item := range feed.Items {
        // Skip if we have enough articles
        if len(articles) >= np.maxArticles {
            break
        }

        // Generate unique ID for article
        articleID := np.generateArticleID(item)
        
        // Check if article exists in database
        existing, err := np.bot.database.GetArticle(articleID)
        if err != nil {
            np.bot.logger.Error("Failed to check article existence: %v", err)
            continue
        }
        if existing != nil {
            continue
        }

        // Create article
        article := &NewsArticle{
            ID:          articleID,
            Title:       html.UnescapeString(item.Title),
            Content:     np.processContent(item),
            URL:         item.Link,
            Source:      source.Name,
            Category:    source.Category,
            PublishedAt: np.getPublishDate(item),
            FetchedAt:   time.Now(),
        }

        // Extract image if available
        if item.Image != nil {
            article.ImageURL = item.Image.URL
        } else if enclosure := np.findImageEnclosure(item); enclosure != nil {
            article.ImageURL = enclosure.URL
        }

        // Perform fact checking if enabled for this source
        if source.FactCheck {
            if result, err := np.bot.factChecker.Check(article); err != nil {
                np.bot.logger.Error("Fact check failed for %s: %v", article.Title, err)
            } else {
                article.FactCheckResult = result
            }
        }

        // Save article to database
        if err := np.bot.database.SaveArticle(article); err != nil {
            np.bot.logger.Error("Failed to save article: %v", err)
            continue
        }

        articles = append(articles, article)
    }

    // Update feed stats on successful processing
    np.updateFeedStats(source, len(articles), nil)

    return articles, nil
}

// generateArticleID creates a unique ID for an article
func (np *NewsProcessor) generateArticleID(item *gofeed.Item) string {
    // Use GUID if available
    if item.GUID != "" {
        return fmt.Sprintf("guid:%x", sha256.Sum256([]byte(item.GUID)))
    }
    
    // Fall back to URL
    if item.Link != "" {
        return fmt.Sprintf("url:%x", sha256.Sum256([]byte(item.Link)))
    }
    
    // Last resort: hash of title and publish date
    data := item.Title
    if item.PublishedParsed != nil {
        data += item.PublishedParsed.String()
    }
    
    hash := sha256.Sum256([]byte(data))
    return fmt.Sprintf("content:%s", hex.EncodeToString(hash[:]))
}

// processContent processes the article content
func (np *NewsProcessor) processContent(item *gofeed.Item) string {
    var content string
    
    // Prefer full content if available
    if item.Content != "" {
        content = item.Content
    } else if item.Description != "" {
        content = item.Description
    } else {
        return item.Title // Fallback to title if no content available
    }

    // Clean content
    content = html.UnescapeString(content)
    content = stripHTML(content)
    content = strings.TrimSpace(content)

    return content
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

// logFeedError logs a feed processing error
func (np *NewsProcessor) logFeedError(source NewsSource, err error) {
    event := &ErrorEvent{
        Component: "NewsProcessor",
        Message:   fmt.Sprintf("Error processing feed %s: %v", source.Name, err),
        Severity:  "ERROR",
        Time:     time.Now(),
    }
    
    if err := np.bot.database.LogError(event); err != nil {
        np.bot.logger.Error("Failed to log feed error: %v", err)
    }
}

// updateFeedStats updates the feed statistics
func (np *NewsProcessor) updateFeedStats(source NewsSource, articleCount int, err error) {
    source.LastFetch = time.Now()
    
    if err == nil {
        np.bot.logger.Info("Successfully processed %d articles from %s", articleCount, source.Name)
    }
    
    if err := np.bot.database.SaveSource(&source); err != nil {
        np.bot.logger.Error("Failed to update feed stats: %v", err)
    }
}
