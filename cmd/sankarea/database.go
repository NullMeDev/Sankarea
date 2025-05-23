// cmd/sankarea/database.go
package main

import (
    "context"
    "database/sql"
    "errors"
    "fmt"
    "time"

    _ "github.com/lib/pq"
    "github.com/jmoiron/sqlx"
)

var (
    db *sqlx.DB
    ErrDatabaseNotInitialized = errors.New("database not initialized")
    ErrDatabaseTimeout       = errors.New("database operation timed out")
)

const (
    defaultQueryTimeout = 10 * time.Second
    maxRetries         = 3
    retryDelay        = time.Second * 2
)

// Database tables
const (
    createArticlesTable = `
    CREATE TABLE IF NOT EXISTS articles (
        id          SERIAL PRIMARY KEY,
        title       TEXT NOT NULL,
        content     TEXT,
        url         TEXT NOT NULL UNIQUE,
        source      TEXT NOT NULL,
        timestamp   TIMESTAMP NOT NULL,
        category    TEXT,
        sentiment   FLOAT,
        fact_score  FLOAT,
        summary     TEXT,
        bias        TEXT,
        topics      TEXT[],
        keywords    TEXT[],
        created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
    )`

    createSourcesTable = `
    CREATE TABLE IF NOT EXISTS sources (
        id              SERIAL PRIMARY KEY,
        name            TEXT NOT NULL UNIQUE,
        url             TEXT NOT NULL,
        category        TEXT,
        description     TEXT,
        bias           TEXT,
        trust_score    FLOAT,
        channel_override TEXT,
        paused         BOOLEAN DEFAULT FALSE,
        active         BOOLEAN DEFAULT TRUE,
        tags           TEXT[],
        last_fetched   TIMESTAMP,
        last_error     TEXT,
        last_error_time TIMESTAMP,
        error_count    INTEGER DEFAULT 0,
        feed_count     INTEGER DEFAULT 0,
        uptime_percent FLOAT DEFAULT 100.0,
        avg_response_time INTEGER DEFAULT 0,
        created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
    )`

    createMetricsTable = `
    CREATE TABLE IF NOT EXISTS metrics (
        id                SERIAL PRIMARY KEY,
        timestamp        TIMESTAMP NOT NULL,
        memory_usage_mb  FLOAT,
        cpu_usage_percent FLOAT,
        disk_usage_percent FLOAT,
        articles_per_minute FLOAT,
        errors_per_hour  FLOAT,
        api_calls_per_hour FLOAT
    )`
)

// initializeDatabase sets up the database connection and schema
func initializeDatabase() error {
    if !cfg.EnableDatabase {
        return nil
    }

    // Build connection string
    connStr := fmt.Sprintf(
        "host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
        cfg.DatabaseHost,
        cfg.DatabasePort,
        cfg.DatabaseUser,
        cfg.DatabasePassword,
        cfg.DatabaseName,
        cfg.DatabaseSSLMode,
    )

    // Connect to database
    var err error
    db, err = sqlx.Connect("postgres", connStr)
    if err != nil {
        return fmt.Errorf("failed to connect to database: %v", err)
    }

    // Set connection pool settings
    db.SetMaxOpenConns(25)
    db.SetMaxIdleConns(5)
    db.SetConnMaxLifetime(5 * time.Minute)

    // Initialize schema
    if err := initializeSchema(); err != nil {
        return fmt.Errorf("failed to initialize schema: %v", err)
    }

    return nil
}

// initializeSchema creates necessary database tables
func initializeSchema() error {
    ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
    defer cancel()

    queries := []string{
        createArticlesTable,
        createSourcesTable,
        createMetricsTable,
    }

    tx, err := db.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %v", err)
    }

    for _, query := range queries {
        if _, err := tx.ExecContext(ctx, query); err != nil {
            tx.Rollback()
            return fmt.Errorf("failed to execute schema query: %v", err)
        }
    }

    return tx.Commit()
}

// storeArticle saves an article to the database
func storeArticle(article *Article) error {
    if !cfg.EnableDatabase || db == nil {
        return ErrDatabaseNotInitialized
    }

    ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
    defer cancel()

    query := `
        INSERT INTO articles (
            title, content, url, source, timestamp, category,
            sentiment, fact_score, summary, bias, topics, keywords
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
        )
        ON CONFLICT (url) DO UPDATE SET
            title = EXCLUDED.title,
            content = EXCLUDED.content,
            sentiment = EXCLUDED.sentiment,
            fact_score = EXCLUDED.fact_score,
            summary = EXCLUDED.summary,
            bias = EXCLUDED.bias,
            topics = EXCLUDED.topics,
            keywords = EXCLUDED.keywords
    `

    _, err := db.ExecContext(ctx, query,
        article.Title,
        article.Content,
        article.URL,
        article.Source,
        article.Timestamp,
        article.Category,
        article.Sentiment,
        article.FactScore,
        article.Summary,
        article.Bias,
        article.Topics,
        article.Keywords,
    )

    if err != nil {
        return fmt.Errorf("failed to store article: %v", err)
    }

    return nil
}

// getRecentArticles retrieves recent articles from the database
func getRecentArticles(limit int) ([]Article, error) {
    if !cfg.EnableDatabase || db == nil {
        return nil, ErrDatabaseNotInitialized
    }

    ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
    defer cancel()

    var articles []Article
    query := `
        SELECT title, content, url, source, timestamp, category,
               sentiment, fact_score, summary, bias, topics, keywords
        FROM articles
        ORDER BY timestamp DESC
        LIMIT $1
    `

    if err := db.SelectContext(ctx, &articles, query, limit); err != nil {
        return nil, fmt.Errorf("failed to fetch recent articles: %v", err)
    }

    return articles, nil
}

// storeMetrics saves system metrics to the database
func storeMetrics(metrics *Metrics) error {
    if !cfg.EnableDatabase || db == nil {
        return ErrDatabaseNotInitialized
    }

    ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
    defer cancel()

    query := `
        INSERT INTO metrics (
            timestamp, memory_usage_mb, cpu_usage_percent,
            disk_usage_percent, articles_per_minute,
            errors_per_hour, api_calls_per_hour
        ) VALUES ($1, $2, $3, $4, $5, $6, $7)
    `

    _, err := db.ExecContext(ctx, query,
        time.Now(),
        metrics.MemoryUsageMB,
        metrics.CPUUsagePercent,
        metrics.DiskUsagePercent,
        metrics.ArticlesPerMinute,
        metrics.ErrorsPerHour,
        metrics.APICallsPerHour,
    )

    if err != nil {
        return fmt.Errorf("failed to store metrics: %v", err)
    }

    return nil
}

// updateSourceStats updates source statistics in the database
func updateSourceStats(source *Source) error {
    if !cfg.EnableDatabase || db == nil {
        return ErrDatabaseNotInitialized
    }

    ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
    defer cancel()

    query := `
        UPDATE sources SET
            last_fetched = $1,
            last_error = $2,
            last_error_time = $3,
            error_count = $4,
            feed_count = $5,
            uptime_percent = $6,
            avg_response_time = $7
        WHERE name = $8
    `

    _, err := db.ExecContext(ctx, query,
        source.LastFetched,
        source.LastError,
        source.LastErrorTime,
        source.ErrorCount,
        source.FeedCount,
        source.UptimePercent,
        source.AvgResponseTime,
        source.Name,
    )

    if err != nil {
        return fmt.Errorf("failed to update source stats: %v", err)
    }

    return nil
}

// cleanupOldData removes old data from the database
func cleanupOldData(retention time.Duration) error {
    if !cfg.EnableDatabase || db == nil {
        return ErrDatabaseNotInitialized
    }

    ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
    defer cancel()

    cutoff := time.Now().Add(-retention)

    queries := []string{
        "DELETE FROM articles WHERE timestamp < $1",
        "DELETE FROM metrics WHERE timestamp < $1",
    }

    tx, err := db.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %v", err)
    }

    for _, query := range queries {
        if _, err := tx.ExecContext(ctx, query, cutoff); err != nil {
            tx.Rollback()
            return fmt.Errorf("failed to cleanup old data: %v", err)
        }
    }

    return tx.Commit()
}
