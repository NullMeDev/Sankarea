// cmd/sankarea/database.go
package main

import (
    "database/sql"
    "encoding/json"
    "fmt"
    "time"
    
    _ "github.com/mattn/go-sqlite3"
)

// Database handles persistent storage operations
type Database struct {
    db *sql.DB
}

// SourceStats holds statistics for a news source
type SourceStats struct {
    URL           string
    Category      string
    LastFetch     time.Time
    TotalArticles int
    ArticlesToday int
}

// NewDatabase creates a new database instance
func NewDatabase(path string) (*Database, error) {
    db, err := sql.Open("sqlite3", path)
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %v", err)
    }

    // Set pragmas for better performance
    pragmas := []string{
        "PRAGMA journal_mode=WAL",
        "PRAGMA synchronous=NORMAL",
        "PRAGMA temp_store=MEMORY",
        "PRAGMA mmap_size=30000000000",
        "PRAGMA cache_size=-2000",
    }

    for _, pragma := range pragmas {
        if _, err := db.Exec(pragma); err != nil {
            db.Close()
            return nil, fmt.Errorf("failed to set pragma: %v", err)
        }
    }

    // Initialize tables
    if err := initializeTables(db); err != nil {
        db.Close()
        return nil, fmt.Errorf("failed to initialize tables: %v", err)
    }

    return &Database{db: db}, nil
}

// initializeTables creates necessary database tables if they don't exist
func initializeTables(db *sql.DB) error {
    tables := []string{
        `CREATE TABLE IF NOT EXISTS articles (
            id TEXT PRIMARY KEY,
            title TEXT NOT NULL,
            content TEXT NOT NULL,
            url TEXT UNIQUE NOT NULL,
            source TEXT NOT NULL,
            category TEXT NOT NULL,
            published_at DATETIME NOT NULL,
            fetched_at DATETIME NOT NULL,
            image_url TEXT,
            citations TEXT,
            fact_check_result TEXT,
            FOREIGN KEY(source) REFERENCES sources(name)
        )`,
        `CREATE TABLE IF NOT EXISTS sources (
            name TEXT PRIMARY KEY,
            url TEXT UNIQUE NOT NULL,
            category TEXT NOT NULL,
            fact_check BOOLEAN NOT NULL DEFAULT 1,
            paused BOOLEAN NOT NULL DEFAULT 0,
            last_fetch DATETIME,
            error_count INTEGER DEFAULT 0,
            last_error TEXT
        )`,
        `CREATE TABLE IF NOT EXISTS errors (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            component TEXT NOT NULL,
            message TEXT NOT NULL,
            severity TEXT NOT NULL,
            timestamp DATETIME NOT NULL
        )`,
        `CREATE INDEX IF NOT EXISTS idx_articles_published ON articles(published_at DESC)`,
        `CREATE INDEX IF NOT EXISTS idx_articles_source ON articles(source)`,
        `CREATE INDEX IF NOT EXISTS idx_articles_category ON articles(category)`,
        `CREATE INDEX IF NOT EXISTS idx_errors_timestamp ON errors(timestamp DESC)`,
    }

    tx, err := db.Begin()
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %v", err)
    }

    for _, table := range tables {
        if _, err := tx.Exec(table); err != nil {
            tx.Rollback()
            return fmt.Errorf("failed to create table: %v", err)
        }
    }

    return tx.Commit()
}

// SaveArticle stores a news article in the database
func (db *Database) SaveArticle(article *NewsArticle) error {
    // Convert citations to JSON if present
    var citationsJSON []byte
    var err error
    if len(article.Citations) > 0 {
        citationsJSON, err = json.Marshal(article.Citations)
        if err != nil {
            return fmt.Errorf("failed to marshal citations: %v", err)
        }
    }

    // Convert fact check result to JSON if present
    var factCheckJSON []byte
    if article.FactCheckResult != nil {
        factCheckJSON, err = json.Marshal(article.FactCheckResult)
        if err != nil {
            return fmt.Errorf("failed to marshal fact check result: %v", err)
        }
    }

    // Insert or update article
    query := `
        INSERT OR REPLACE INTO articles (
            id, title, content, url, source, category,
            published_at, fetched_at, image_url, citations, fact_check_result
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `

    tx, err := db.db.Begin()
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %v", err)
    }

    _, err = tx.Exec(query,
        article.ID,
        article.Title,
        article.Content,
        article.URL,
        article.Source,
        article.Category,
        article.PublishedAt,
        article.FetchedAt,
        article.ImageURL,
        citationsJSON,
        factCheckJSON,
    )

    if err != nil {
        tx.Rollback()
        return fmt.Errorf("failed to save article: %v", err)
    }

    return tx.Commit()
}

// GetArticle retrieves an article by its ID
func (db *Database) GetArticle(id string) (*NewsArticle, error) {
    query := `
        SELECT id, title, content, url, source, category, 
               published_at, fetched_at, image_url, citations, fact_check_result
        FROM articles 
        WHERE id = ?
    `
    
    var article NewsArticle
    var citationsJSON, factCheckJSON sql.NullString
    
    err := db.db.QueryRow(query, id).Scan(
        &article.ID,
        &article.Title,
        &article.Content,
        &article.URL,
        &article.Source,
        &article.Category,
        &article.PublishedAt,
        &article.FetchedAt,
        &article.ImageURL,
        &citationsJSON,
        &factCheckJSON,
    )

    if err == sql.ErrNoRows {
        return nil, nil
    }
    if err != nil {
        return nil, fmt.Errorf("failed to get article: %v", err)
    }

    // Parse citations if present
    if citationsJSON.Valid && citationsJSON.String != "" {
        if err := json.Unmarshal([]byte(citationsJSON.String), &article.Citations); err != nil {
            return nil, fmt.Errorf("failed to unmarshal citations: %v", err)
        }
    }

    // Parse fact check result if present
    if factCheckJSON.Valid && factCheckJSON.String != "" {
        article.FactCheckResult = &FactCheckResult{}
        if err := json.Unmarshal([]byte(factCheckJSON.String), article.FactCheckResult); err != nil {
            return nil, fmt.Errorf("failed to unmarshal fact check result: %v", err)
        }
    }

    return &article, nil
}

// SaveSource stores or updates a news source in the database
func (db *Database) SaveSource(source *NewsSource) error {
    query := `
        INSERT OR REPLACE INTO sources (
            name, url, category, fact_check, paused, last_fetch, error_count, last_error
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
    `

    _, err := db.db.Exec(query,
        source.Name,
        source.URL,
        source.Category,
        source.FactCheck,
        source.Paused,
        source.LastFetch,
        source.ErrorCount,
        source.LastError,
    )

    if err != nil {
        return fmt.Errorf("failed to save source: %v", err)
    }

    return nil
}

// GetSources retrieves all news sources
func (db *Database) GetSources() ([]NewsSource, error) {
    query := `
        SELECT name, url, category, fact_check, paused, last_fetch, error_count, last_error
        FROM sources
    `
    
    rows, err := db.db.Query(query)
    if err != nil {
        return nil, fmt.Errorf("failed to query sources: %v", err)
    }
    defer rows.Close()

    var sources []NewsSource
    for rows.Next() {
        var source NewsSource
        var lastFetch, lastError sql.NullTime
        if err := rows.Scan(
            &source.Name,
            &source.URL,
            &source.Category,
            &source.FactCheck,
            &source.Paused,
            &lastFetch,
            &source.ErrorCount,
            &source.LastError,
        ); err != nil {
            return nil, fmt.Errorf("failed to scan source: %v", err)
        }

        if lastFetch.Valid {
            source.LastFetch = lastFetch.Time
        }

        sources = append(sources, source)
    }

    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("error iterating sources: %v", err)
    }

    return sources, nil
}

// LogError stores an error event in the database
func (db *Database) LogError(event *ErrorEvent) error {
    query := `
        INSERT INTO errors (
            component, message, severity, timestamp
        ) VALUES (?, ?, ?, ?)
    `

    _, err := db.db.Exec(query,
        event.Component,
        event.Message,
        event.Severity,
        event.Time,
    )

    if err != nil {
        return fmt.Errorf("failed to log error: %v", err)
    }

    return nil
}

// GetRecentErrors retrieves recent error events
func (db *Database) GetRecentErrors(limit int) ([]*ErrorEvent, error) {
    query := `
        SELECT component, message, severity, timestamp
        FROM errors
        ORDER BY timestamp DESC
        LIMIT ?
    `

    rows, err := db.db.Query(query, limit)
    if err != nil {
        return nil, fmt.Errorf("failed to query errors: %v", err)
    }
    defer rows.Close()

    var events []*ErrorEvent
    for rows.Next() {
        event := &ErrorEvent{}
        if err := rows.Scan(
            &event.Component,
            &event.Message,
            &event.Severity,
            &event.Time,
        ); err != nil {
            return nil, fmt.Errorf("failed to scan error: %v", err)
        }
        events = append(events, event)
    }

    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("error iterating errors: %v", err)
    }

    return events, nil
}

// GetSourceStats retrieves statistics for all sources
func (db *Database) GetSourceStats() (map[string]SourceStats, error) {
    query := `
        SELECT 
            s.name,
            s.url,
            s.category,
            s.last_fetch,
            COUNT(a.id) as article_count,
            SUM(CASE WHEN a.fetched_at > datetime('now', '-24 hours') THEN 1 ELSE 0 END) as today_count
        FROM sources s
        LEFT JOIN articles a ON s.name = a.source
        GROUP BY s.name, s.url, s.category, s.last_fetch
    `
    
    rows, err := db.db.Query(query)
    if err != nil {
        return nil, fmt.Errorf("failed to query source stats: %v", err)
    }
    defer rows.Close()

    stats := make(map[string]SourceStats)
    for rows.Next() {
        var s SourceStats
        var name string
        var lastFetch sql.NullTime
        if err := rows.Scan(&name, &s.URL, &s.Category, &lastFetch, &s.TotalArticles, &s.ArticlesToday); err != nil {
            return nil, fmt.Errorf("failed to scan source stats: %v", err)
        }
        if lastFetch.Valid {
            s.LastFetch = lastFetch.Time
        }
        stats[name] = s
    }

    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("error iterating source stats: %v", err)
    }

    return stats, nil
}

// CleanOldArticles removes articles older than the specified duration
func (db *Database) CleanOldArticles(age time.Duration) error {
    query := `DELETE FROM articles WHERE published_at < datetime('now', '-' || ? || ' seconds')`
    
    result, err := db.db.Exec(query, int64(age.Seconds()))
    if err != nil {
        return fmt.Errorf("failed to clean old articles: %v", err)
    }

    rows, err := result.RowsAffected()
    if err != nil {
        return fmt.Errorf("failed to get affected rows: %v", err)
    }

    return nil
}

// CleanOldErrors removes error logs older than the specified duration
func (db *Database) CleanOldErrors(age time.Duration) error {
    query := `DELETE FROM errors WHERE timestamp < datetime('now', '-' || ? || ' seconds')`
    
    result, err := db.db.Exec(query, int64(age.Seconds()))
    if err != nil {
        return fmt.Errorf("failed to clean old errors: %v", err)
    }

    rows, err := result.RowsAffected()
    if err != nil {
        return fmt.Errorf("failed to get affected rows: %v", err)
    }

    return nil
}

// GetArticleCount returns the total number of articles in the database
func (db *Database) GetArticleCount() (int, error) {
    var count int
    err := db.db.QueryRow("SELECT COUNT(*) FROM articles").Scan(&count)
    if err != nil {
        return 0, fmt.Errorf("failed to get article count: %v", err)
    }
    return count, nil
}

// GetRecentArticles retrieves recent articles with optional filtering
func (db *Database) GetRecentArticles(limit int, category string) ([]*NewsArticle, error) {
    query := `
        SELECT id, title, content, url, source, category, 
               published_at, fetched_at, image_url, citations, fact_check_result
        FROM articles
        WHERE category = COALESCE(?, category)
        ORDER BY published_at DESC
        LIMIT ?
    `
    
    rows, err := db.db.Query(query, category, limit)
    if err != nil {
        return nil, fmt.Errorf("failed to query recent articles: %v", err)
    }
    defer rows.Close()

    var articles []*NewsArticle
    for rows.Next() {
        var article NewsArticle
        var citationsJSON, factCheckJSON sql.NullString
        
        if err := rows.Scan(
            &article.ID,
            &article.Title,
            &article.Content,
            &article.URL,
            &article.Source,
            &article.Category,
            &article.PublishedAt,
            &article.FetchedAt,
            &article.ImageURL,
            &citationsJSON,
            &factCheckJSON,
        ); err != nil {
            return nil, fmt.Errorf("failed to scan article: %v", err)
        }

        // Parse citations if present
        if citationsJSON.Valid && citationsJSON.String != "" {
            if err := json.Unmarshal([]byte(citationsJSON.String), &article.Citations); err != nil {
                return nil, fmt.Errorf("failed to unmarshal citations: %v", err)
            }
        }

        // Parse fact check result if present
        if factCheckJSON.Valid && factCheckJSON.String != "" {
            article.FactCheckResult = &FactCheckResult{}
            if err := json.Unmarshal([]byte(factCheckJSON.String), article.FactCheckResult); err != nil {
                return nil, fmt.Errorf("failed to unmarshal fact check result: %v", err)
            }
        }

        articles = append(articles, &article)
    }

    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("error iterating articles: %v", err)
    }

    return articles, nil
}

// Close closes the database connection
func (db *Database) Close() error {
    return db.db.Close()
}
