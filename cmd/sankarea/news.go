// cmd/sankarea/news.go
package main

import (
    "context"
    "fmt"
    "net/http"
    "strings"
    "sync"
    "time"

    "github.com/mmcdole/gofeed"
)

const (
    maxConcurrentFeeds = 5
    feedTimeout        = 30 * time.Second
    maxRetries        = 3
    retryDelay        = 5 * time.Second
)

// NewsProcessor handles news feed processing
type NewsProcessor struct {
    client    *http.Client
    parser    *gofeed.Parser
    mutex     sync.RWMutex
    cache     map[string]time.Time
    semaphore chan struct{}
}

// NewNewsProcessor creates a new NewsProcessor instance
func NewNewsProcessor() *NewsProcessor {
    return &NewsProcessor{
        client: &http.Client{
            Timeout: feedTimeout,
        },
        parser:    gofeed.NewParser(),
        cache:     make(map[string]time.Time),
        semaphore: make(chan struct{}, maxConcurrentFeeds),
    }
}

// fetchNewsWithContext fetches news with context support
func fetchNewsWithContext(ctx context.Context) error {
    processor := NewNewsProcessor()
    sources := loadSources()

    // Filter active sources
    activeSources := make([]Source, 0)
    for _, source := range sources {
        if !source.Paused && source.Active {
            activeSources = append(activeSources, source)
        }
    }

    if len(activeSources) == 0 {
        return fmt.Errorf("no active sources configured")
    }

    // Create error channel for collecting errors
    errCh := make(chan error, len(activeSources))
    var wg sync.WaitGroup

    // Process each source
    for _, source := range activeSources {
        wg.Add(1)
        go func(src Source) {
            defer wg.Done()
            if err := processor.processFeed(ctx, src); err != nil {
                errCh <- fmt.Errorf("error processing %s: %v", src.Name, err)
                
                // Update source error stats
                updateSourceError(src.Name, err)
            }
        }(source)
    }

    // Wait for all goroutines to finish
    go func() {
        wg.Wait()
        close(errCh)
    }()

    // Collect errors
    var errors []string
    for err := range errCh {
        errors = append(errors, err.Error())
    }

    // Update state after fetch
    if err := UpdateState(func(s *State) {
        s.LastFetchTime = time.Now()
        s.FeedCount += len(activeSources)
        if len(errors) > 0 {
            s.LastError = strings.Join(errors, "; ")
            s.LastErrorTime = time.Now()
            s.ErrorCount += len(errors)
        }
    }); err != nil {
        Logger().Printf("Failed to update state after news fetch: %v", err)
    }

    if len(errors) > 0 {
        return fmt.Errorf("encountered errors while fetching news: %s", strings.Join(errors, "; "))
    }

    return nil
}

// processFeed handles fetching and processing a single feed
func (np *NewsProcessor) processFeed(ctx context.Context, source Source) error {
    // Acquire semaphore
    select {
    case np.semaphore <- struct{}{}:
        defer func() { <-np.semaphore }()
    case <-ctx.Done():
        return ctx.Err()
    }

    // Check cache to avoid duplicate processing
    np.mutex.RLock()
    lastFetch, exists := np.cache[source.URL]
    np.mutex.RUnlock()

    if exists && time.Since(lastFetch) < time.Duration(cfg.NewsIntervalMinutes)*time.Minute {
        return nil
    }

    // Fetch and process feed with retries
    var feed *gofeed.Feed
    var err error
    for attempt := 1; attempt <= maxRetries; attempt++ {
        feed, err = np.fetchFeedWithContext(ctx, source)
        if err == nil {
            break
        }
        if attempt < maxRetries {
            select {
            case <-ctx.Done():
                return ctx.Err()
            case <-time.After(retryDelay):
                continue
            }
        }
    }
    if err != nil {
        return fmt.Errorf("failed to fetch feed after %d attempts: %v", maxRetries, err)
    }

    // Process feed items
    if err := np.processItems(ctx, source, feed); err != nil {
        return fmt.Errorf("failed to process items: %v", err)
    }

    // Update cache
    np.mutex.Lock()
    np.cache[source.URL] = time.Now()
    np.mutex.Unlock()

    return nil
}

// fetchFeedWithContext fetches a feed with context support
func (np *NewsProcessor) fetchFeedWithContext(ctx context.Context, source Source) (*gofeed.Feed, error) {
    req, err := http.NewRequestWithContext(ctx, "GET", source.URL, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %v", err)
    }

    // Set user agent
    req.Header.Set("User-Agent", cfg.UserAgentString)

    resp, err := np.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("failed to fetch feed: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("received status code %d", resp.StatusCode)
    }

    feed, err := np.parser.Parse(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to parse feed: %v", err)
    }

    return feed, nil
}

// processItems handles processing feed items
func (np *NewsProcessor) processItems(ctx context.Context, source Source, feed *gofeed.Feed) error {
    if feed == nil || len(feed.Items) == 0 {
        return nil
    }

    // Get most recent items up to configured limit
    itemLimit := cfg.MaxPostsPerSource
    if itemLimit > len(feed.Items) {
        itemLimit = len(feed.Items)
    }

    recentItems := feed.Items[:itemLimit]

    // Process each item
    for _, item := range recentItems {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            if err := processNewsItem(ctx, source, item); err != nil {
                Logger().Printf("Error processing item from %s: %v", source.Name, err)
                continue
            }
        }
    }

    return nil
}

// updateSourceError updates error statistics for a source
func updateSourceError(sourceName string, err error) {
    sources := loadSources()
    for i, src := range sources {
        if src.Name == sourceName {
            sources[i].LastError = err.Error()
            sources[i].LastErrorTime = time.Now()
            sources[i].ErrorCount++
            saveSources(sources)
            break
        }
    }
}

// Helper function to process individual news items
func processNewsItem(ctx context.Context, source Source, item *gofeed.Item) error {
    // Convert feed item to our Article type
    article := Article{
        Title:     item.Title,
        Content:   item.Description,
        URL:       item.Link,
        Source:    source.Name,
        Timestamp: *item.PublishedParsed,
    }

    // Perform content analysis if enabled
    if cfg.EnableFactCheck || cfg.EnableSummarization {
        if err := analyzeContent(ctx, &article); err != nil {
            Logger().Printf("Content analysis failed for %s: %v", article.Title, err)
        }
    }

    // Store article if database is enabled
    if cfg.EnableDatabase {
        if err := storeArticle(&article); err != nil {
            return fmt.Errorf("failed to store article: %v", err)
        }
    }

    return nil
}
