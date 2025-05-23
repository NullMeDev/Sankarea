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

// NewDatabase creates a new database instance
func NewDatabase(path string) (*Database, error) {
    db, err := sql.Open("sqlite3", path)
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %v", err)
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
            citations TEXT,
            fact_check_result TEXT
        )`,
        `CREATE TABLE IF NOT EXISTS sources (
            name TEXT PRIMARY KEY,
            url TEXT UNIQUE NOT NULL,
            category TEXT NOT NULL,
            fact_check BOOLEAN NOT NULL DEFAULT 1,
            paused BOOLEAN NOT NULL DEFAULT 0,
            last_fetch DATETIME
        )`,
        `CREATE TABLE IF NOT EXISTS errors (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            component TEXT NOT NULL,
            message TEXT NOT NULL,
            severity TEXT NOT NULL,
            timestamp DATETIME NOT NULL
        )`,
    }

    for _, table := range tables {
        if _, err := db.Exec(table); err != nil {
            return fmt.Errorf("failed to create table: %v", err)
        }
    }

    return nil
}

// SaveArticle stores a news article in the database
func (db *Database) SaveArticle(article *NewsArticle) error {
    // Convert citations to JSON
    citationsJSON, err := json.Marshal(article.Citations)
    if err != nil {
        return fmt.Errorf("failed to marshal citations: %v", err)
    }

    // Convert fact check result to JSON
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
            published_at, fetched_at, citations, fact_check_result
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `

    _, err = db.db.Exec(query,
        article.ID,
        article.Title,
        article.Content,
        article.URL,
        article.Source,
        article.Category,
        article.PublishedAt,
        article.FetchedAt,
        string(citationsJSON),
        string(factCheckJSON),
    )

    if err != nil {
        return fmt.Errorf("failed to save article: %v", err)
    }

    return nil
}

// GetArticle retrieves an article by its ID
func (db *Database) GetArticle(id string) (*NewsArticle, error) {
    query := `SELECT * FROM articles WHERE id = ?`
    
    var article NewsArticle
    var citationsJSON, factCheckJSON string
    
    err := db.db.QueryRow(query, id).Scan(
        &article.ID,
        &article.Title,
        &article.Content,
        &article.URL,
        &article.Source,
        &article.Category,
        &article.PublishedAt,
        &article.FetchedAt,
        &citationsJSON,
        &factCheckJSON,
    )

    if err == sql.ErrNoRows {
        return nil, nil
    }
    if err != nil {
        return nil, fmt.Errorf("failed to get article: %v", err)
    }

    // Parse citations
    if err := json.Unmarshal([]byte(citationsJSON), &article.Citations); err != nil {
        return nil, fmt.Errorf("failed to unmarshal citations: %v", err)
    }

    // Parse fact check result if exists
    if factCheckJSON != "" {
        article.FactCheckResult = &FactCheckResult{}
        if err := json.Unmarshal([]byte(factCheckJSON), article.FactCheckResult); err != nil {
            return nil, fmt.Errorf("failed to unmarshal fact check result: %v", err)
        }
    }

    return &article, nil
}

// SaveSource stores a news source in the database
func (db *Database) SaveSource(source *NewsSource) error {
    query := `
        INSERT OR REPLACE INTO sources (
            name, url, category, fact_check, paused, last_fetch
        ) VALUES (?, ?, ?, ?, ?, ?)
    `

    _, err := db.db.Exec(query,
        source.Name,
        source.URL,
        source.Category,
        source.FactCheck,
        source.Paused,
        time.Now(),
    )

    if err != nil {
        return fmt.Errorf("failed to save source: %v", err)
    }

    return nil
}

// GetSources retrieves all news sources
func (db *Database) GetSources() ([]NewsSource, error) {
    query := `SELECT name, url, category, fact_check, paused FROM sources`
    
    rows, err := db.db.Query(query)
    if err != nil {
        return nil, fmt.Errorf("failed to query sources: %v", err)
    }
    defer rows.Close()

    var sources []NewsSource
    for rows.Next() {
        var source NewsSource
        if err := rows.Scan(
            &source.Name,
            &source.URL,
            &source.Category,
            &source.FactCheck,
            &source.Paused,
        ); err != nil {
            return nil, fmt.Errorf("failed to scan source: %v", err)
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

// Close closes the database connection
func (db *Database) Close() error {
    return db.db.Close()
}
