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
    "sync"
    "time"

    "github.com/mmcdole/gofeed"
)

// NewsArticle represents a processed news article
type NewsArticle struct {
    ID             string           `json:"id"`
    Title          string           `json:"title"`
    Content        string           `json:"content"`
    URL            string           `json:"url"`
    ImageURL       string           `json:"image_url,omitempty"`
    Source         string           `json:"source"`
    Category       string           `json:"category"`
    PublishedAt    time.Time       `json:"published_at"`
    FetchedAt      time.Time       `json:"fetched_at"`
    Citations      []string        `json:"citations,omitempty"`
    FactCheckResult *FactCheckResult `json:"fact_check_result,omitempty"`
}

// NewsProcessor handles the fetching and processing of RSS feeds
type NewsProcessor struct {
    parser      *gofeed.Parser
    client      *http.Client
    bot         *Bot
    maxArticles int
    userAgent   string
    timeout     time.Duration
    mu          sync.RWMutex
}

// NewNewsProcessor creates a new NewsProcessor instance
func NewNewsProcessor(bot *Bot) *NewsProcessor {
    return &NewsProcessor{
        parser: gofeed.NewParser(),
        client: &http.Client{
            Timeout: 30 * time.Second,
            Transport: &http.Transport{
                MaxIdleConns:        100,
                IdleConnTimeout:     90 * time.Second,
                DisableCompression:  true,
                MaxConnsPerHost:     10,
                DisableKeepAlives:   false,
                ForceAttemptHTTP2:   true,
            },
        },
        bot:         bot,
        maxArticles: 5,
        userAgent:   "Sankarea News Bot/1.0",
        timeout:     30 * time.Second,
    }
}

// ProcessFeeds fetches and processes all enabled feeds
func (np *NewsProcessor) ProcessFeeds(ctx context.Context, sources []NewsSource) ([]*NewsArticle, error) {
    var (
        articles = make([]*NewsArticle, 0)
        errors   = make([]error, 0)
        mu       sync.Mutex
        wg       sync.WaitGroup
    )

    // Create semaphore for concurrent processing
    sem := make(chan struct{}, 5) // limit to 5 concurrent requests

    // Process each source
    for _, source := range sources {
        if source.Paused {
            np.bot.logger.Info("Skipping paused source: %s", source.Name)
            continue
        }

        wg.Add(1)
        go func(src NewsSource) {
            defer wg.Done()
            sem <- struct{}{} // acquire semaphore
            defer func() { <-sem }() // release semaphore

            // Process the feed with timeout
            feedCtx, cancel := context.WithTimeout(ctx, np.timeout)
            defer cancel()

            feedArticles, err := np.processFeed(feedCtx, src)
            mu.Lock()
            if err != nil {
                np.bot.logger.Error("Failed to process %s: %v", src.Name, err)
                errors = append(errors, fmt.Errorf("error processing %s: %v", src.Name, err))
            } else {
                articles = append(articles, feedArticles...)
            }
            mu.Unlock()
        }(source)
    }

    // Wait for all goroutines to complete
    wg.Wait()

    // Check for errors
    if len(errors) > 0 {
        var errMsgs []string
        for _, err := range errors {
            errMsgs = append(errMsgs, err.Error())
        }
        return articles, fmt.Errorf("feed processing errors: %s", strings.Join(errMsgs, "; "))
    }

    return articles, nil
}

// processFeed fetches and processes a single feed
func (np *NewsProcessor) processFeed(ctx context.Context, source NewsSource) ([]*NewsArticle, error) {
    // Create request with context and timeout
    req, err := http.NewRequestWithContext(ctx, "GET", source.URL, nil)
    if err != nil {
        np.logFeedError(source, err)
        return nil, fmt.Errorf("failed to create request: %v", err)
    }

    // Set headers
    req.Header.Set("User-Agent", np.userAgent)
    req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml")

    // Perform request
    resp, err := np.client.Do(req)
    if err != nil {
        np.logFeedError(source, err)
        return nil, fmt.Errorf("failed to fetch feed: %v", err)
    }
    defer resp.Body.Close()

    // Check response status
    if resp.StatusCode != http.StatusOK {
        err := fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
        np.logFeedError(source, err)
        return nil, err
    }

    // Read response with timeout
    bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // 10MB limit
    if err != nil {
        np.logFeedError(source, err)
        return nil, fmt.Errorf("failed to read response: %v", err)
    }

    // Parse feed
    feed, err := np.parser.ParseString(string(bodyBytes))
    if err != nil {
        np.logFeedError(source, err)
        return nil, fmt.Errorf("failed to parse feed: %v", err)
    }

    // Process articles
    var articles []*NewsArticle
    seenURLs := make(map[string]bool)

    for _, item := range feed.Items {
        // Skip if we have enough articles
        if len(articles) >= np.maxArticles {
            break
        }

        // Generate article ID
        articleID := np.generateArticleID(item)

        // Skip if article exists in database
        exists, err := np.articleExists(articleID)
        if err != nil {
            np.bot.logger.Error("Failed to check article existence: %v", err)
            continue
        }
        if exists {
            continue
        }

        // Skip duplicate URLs in current batch
        if seenURLs[item.Link] {
            continue
        }
        seenURLs[item.Link] = true

        // Create article
        article := &NewsArticle{
            ID:          articleID,
            Title:       html.UnescapeString(item.Title),
            Content:     np.processContent(item),
            URL:         item.Link,
            Source:      source.Name,
            Category:    source.Category,
            PublishedAt: np.getPublishDate(item),
            FetchedAt:   time.Now().UTC(),
        }

        // Extract image
        if imageURL := np.extractImage(item); imageURL != "" {
            article.ImageURL = imageURL
        }

        // Perform fact checking if enabled
        if source.FactCheck {
            if result, err := np.bot.factChecker.Check(article); err != nil {
                np.bot.logger.Error("Fact check failed for %s: %v", article.Title, err)
            } else {
                article.FactCheckResult = result
            }
        }

        // Extract citations
        article.Citations = np.extractCitations(item)

        // Save article to database
        if err := np.bot.database.SaveArticle(article); err != nil {
            np.bot.logger.Error("Failed to save article: %v", err)
            continue
        }

        articles = append(articles, article)
    }

    // Update feed stats
    np.updateFeedStats(source, len(articles), nil)

    return articles, nil
}

// articleExists checks if an article already exists in the database
func (np *NewsProcessor) articleExists(id string) (bool, error) {
    article, err := np.bot.database.GetArticle(id)
    if err != nil {
        return false, err
    }
    return article != nil, nil
}

// generateArticleID creates a unique ID for an article
func (np *NewsProcessor) generateArticleID(item *gofeed.Item) string {
    // Use GUID if available
    if item.GUID != "" {
        hash := sha256.Sum256([]byte(item.GUID))
        return fmt.Sprintf("guid:%s", hex.EncodeToString(hash[:]))
    }
    
    // Use URL if available
    if item.Link != "" {
        hash := sha256.Sum256([]byte(item.Link))
        return fmt.Sprintf("url:%s", hex.EncodeToString(hash[:]))
    }
    
    // Fallback to title and date combination
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

    // Truncate if too long (Discord limit is 2048 characters)
    if len(content) > 2000 {
        content = content[:1997] + "..."
    }

    return content
}

// extractImage finds the best image for an article
func (np *NewsProcessor) extractImage(item *gofeed.Item) string {
    // Check feed item image
    if item.Image != nil && item.Image.URL != "" {
        return item.Image.URL
    }

    // Check enclosures
    for _, enc := range item.Enclosures {
        if strings.HasPrefix(enc.Type, "image/") {
            return enc.URL
        }
    }

    // Check ITunes image
    if item.ITunesExt != nil && item.ITunesExt.Image != "" {
        return item.ITunesExt.Image
    }

    return ""
}

// extractCitations extracts citation links from the content
func (np *NewsProcessor) extractCitations(item *gofeed.Item) []string {
    var citations []string
    seen := make(map[string]bool)

    // Add main link
    if item.Link != "" {
        citations = append(citations, item.Link)
        seen[item.Link] = true
    }

    // Add links from extensions
    for _, ext := range item.Extensions {
        for _, exts := range ext {
            for _, e := range exts {
                for _, link := range e.Attrs {
                    if strings.HasPrefix(link.Value, "http") && !seen[link.Value] {
                        citations = append(citations, link.Value)
                        seen[link.Value] = true
                    }
                }
            }
        }
    }

    return citations
}

// getPublishDate gets the publish date of an article
func (np *NewsProcessor) getPublishDate(item *gofeed.Item) time.Time {
    if item.PublishedParsed != nil {
        return item.PublishedParsed.UTC()
    }
    if item.UpdatedParsed != nil {
        return item.UpdatedParsed.UTC()
    }
    return time.Now().UTC()
}

// logFeedError logs a feed processing error
func (np *NewsProcessor) logFeedError(source NewsSource, err error) {
    event := &ErrorEvent{
        Component: "NewsProcessor",
        Message:   fmt.Sprintf("Error processing feed %s: %v", source.Name, err),
        Severity:  "ERROR",
        Time:     time.Now().UTC(),
    }
    
    if err := np.bot.database.LogError(event); err != nil {
        np.bot.logger.Error("Failed to log feed error: %v", err)
    }
}

// updateFeedStats updates the feed statistics
func (np *NewsProcessor) updateFeedStats(source NewsSource, articleCount int, err error) {
    source.LastFetch = time.Now().UTC()
    
    if err != nil {
        source.ErrorCount++
        source.LastError = err.Error()
    } else {
        source.ErrorCount = 0
        source.LastError = ""
        np.bot.logger.Info("Successfully processed %d articles from %s", articleCount, source.Name)
    }
    
    if err := np.bot.database.SaveSource(&source); err != nil {
        np.bot.logger.Error("Failed to update feed stats: %v", err)
    }
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
