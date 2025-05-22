package main

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

// InitDB initializes the SQLite database
func InitDB() error {
	var err error
	db, err = sql.Open("sqlite3", "data/sankarea.db")
	if err != nil {
		return err
	}

	// Create tables if they don't exist
	createTables := []string{
		`CREATE TABLE IF NOT EXISTS articles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			url TEXT UNIQUE NOT NULL,
			source_name TEXT NOT NULL,
			published_at DATETIME NOT NULL,
			fetched_at DATETIME NOT NULL,
			content TEXT,
			summary TEXT,
			sentiment REAL DEFAULT 0,
			fact_check_score REAL DEFAULT 0,
			category TEXT,
			channel_id TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS fact_checks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			article_id INTEGER NOT NULL,
			claim TEXT NOT NULL,
			rating TEXT NOT NULL,
			source TEXT NOT NULL,
			checked_at DATETIME NOT NULL,
			FOREIGN KEY(article_id) REFERENCES articles(id)
		)`,
		`CREATE TABLE IF NOT EXISTS analytics (
			date TEXT PRIMARY KEY,
			articles_processed INTEGER DEFAULT 0,
			messages_sent INTEGER DEFAULT 0,
			api_calls INTEGER DEFAULT 0,
			api_cost REAL DEFAULT 0,
			errors INTEGER DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS user_preferences (
			user_id TEXT PRIMARY KEY,
			saved_articles TEXT,
			preferred_categories TEXT,
			preferred_sources TEXT,
			notification_enabled BOOLEAN DEFAULT 0,
			theme TEXT DEFAULT 'light'
		)`,
		`CREATE TABLE IF NOT EXISTS topic_mentions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			topic_name TEXT NOT NULL,
			article_id INTEGER NOT NULL,
			mention_count INTEGER DEFAULT 1,
			mentioned_at DATETIME NOT NULL,
			FOREIGN KEY(article_id) REFERENCES articles(id)
		)`,
	}

	for _, query := range createTables {
		_, err := db.Exec(query)
		if err != nil {
			return err
		}
	}

	return nil
}

// CloseDB closes the database connection
func CloseDB() error {
	if db != nil {
		return db.Close()
	}
	return nil
}

// AddArticle adds a new article to the database
func AddArticle(title, url, sourceName, content, summary, category, channelID string, published time.Time, sentiment, factScore float64) (int64, error) {
	query := `INSERT INTO articles (
		title, url, source_name, published_at, fetched_at, content, summary, 
		sentiment, fact_check_score, category, channel_id
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := db.Exec(query, 
		title, url, sourceName, published, time.Now(), content, summary, 
		sentiment, factScore, category, channelID)
	if err != nil {
		return 0, err
	}

	// Get the new article ID
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	// Update daily analytics
	updateDailyAnalytics("articles_processed", 1)

	return id, nil
}

// GetArticleByURL retrieves an article by its URL
func GetArticleByURL(url string) (int64, bool, error) {
	var id int64
	query := "SELECT id FROM articles WHERE url = ?"
	err := db.QueryRow(query, url).Scan(&id)
	
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	
	if err != nil {
		return 0, false, err
	}
	
	return id, true, nil
}

// AddFactCheck adds a fact check result to the database
func AddFactCheck(articleID int64, claim, rating, source string) error {
	query := `INSERT INTO fact_checks (
		article_id, claim, rating, source, checked_at
	) VALUES (?, ?, ?, ?, ?)`

	_, err := db.Exec(query, 
		articleID, claim, rating, source, time.Now())
	
	return err
}

// GetArticlesByCategory gets articles by category within a time range
func GetArticlesByCategory(category string, hours int) ([]ArticleDigest, error) {
	query := `
		SELECT 
			title, url, source_name, published_at, category
		FROM 
			articles
		WHERE 
			category = ? AND published_at > ?
		ORDER BY 
			published_at DESC
	`
	
	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	rows, err := db.Query(query, category, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var articles []ArticleDigest
	for rows.Next() {
		var article ArticleDigest
		var publishedStr string
		
		err := rows.Scan(
			&article.Title,
			&article.URL,
			&article.Source,
			&publishedStr,
			&article.Category,
		)
		if err != nil {
			return nil, err
		}
		
		// Parse published time
		article.Published, _ = time.Parse("2006-01-02 15:04:05", publishedStr)
		
		articles = append(articles, article)
	}
	
	if err = rows.Err(); err != nil {
		return nil, err
	}
	
	return articles, nil
}

// GetTrendingTopics gets trending topics from recent articles
func GetTrendingTopics(hours int) ([]struct{ Topic string; Count int }, error) {
	query := `
		SELECT 
			topic_name, SUM(mention_count) as total_mentions
		FROM 
			topic_mentions
		JOIN 
			articles ON topic_mentions.article_id = articles.id
		WHERE 
			mentioned_at > ?
		GROUP BY 
			topic_name
		ORDER BY 
			total_mentions DESC
		LIMIT 10
	`
	
	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	rows, err := db.Query(query, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var topics []struct{ Topic string; Count int }
	for rows.Next() {
		var topic struct{ Topic string; Count int }
		
		err := rows.Scan(&topic.Topic, &topic.Count)
		if err != nil {
			return nil, err
		}
		
		topics = append(topics, topic)
	}
	
	if err = rows.Err(); err != nil {
		return nil, err
	}
	
	return topics, nil
}

// SaveUserPreference saves a user preference
func SaveUserPreference(userID, prefType, value string) error {
	query := `
		INSERT INTO user_preferences (user_id, ` + prefType + `)
		VALUES (?, ?)
		ON CONFLICT(user_id) DO UPDATE SET ` + prefType + ` = ?
	`
	
	_, err := db.Exec(query, userID, value, value)
	return err
}

// GetUserPreference gets a user preference
func GetUserPreference(userID, prefType string) (string, error) {
	query := `SELECT ` + prefType + ` FROM user_preferences WHERE user_id = ?`
	
	var value string
	err := db.QueryRow(query, userID).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil // No preference set
	}
	
	return value, err
}

// updateDailyAnalytics updates analytics for today
func updateDailyAnalytics(metric string, increment int) error {
	if db == nil {
		return nil // No database
	}
	
	date := time.Now().Format("2006-01-02")
	query := `
		INSERT INTO analytics (date, ` + metric + `)
		VALUES (?, ?)
		ON CONFLICT(date) DO UPDATE SET ` + metric + ` = ` + metric + ` + ?
	`
	
	_, err := db.Exec(query, date, increment, increment)
	return err
}

// GetAnalytics gets analytics for a date range
func GetAnalytics(startDate, endDate time.Time) (map[string]map[string]int, error) {
	query := `
		SELECT 
			date, 
			articles_processed, 
			messages_sent,
			api_calls, 
			errors
		FROM 
			analytics
		WHERE 
			date BETWEEN ? AND ?
		ORDER BY 
			date
	`
	
	rows, err := db.Query(query, 
		startDate.Format("2006-01-02"), 
		endDate.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	result := make(map[string]map[string]int)
	
	for rows.Next() {
		var date string
		var articlesProcessed, messagesSent, apiCalls, errors int
		
		err := rows.Scan(&date, &articlesProcessed, &messagesSent, &apiCalls, &errors)
		if err != nil {
			return nil, err
		}
		
		result[date] = map[string]int{
			"articles_processed": articlesProcessed,
			"messages_sent":     messagesSent,
			"api_calls":         apiCalls,
			"errors":           errors,
		}
	}
	
	if err = rows.Err(); err != nil {
		return nil, err
	}
	
	return result, nil
}
