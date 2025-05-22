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

	_, err := db.Exec(query, articleID, claim, rating, source, time.Now())
	return err
}

// updateDailyAnalytics updates the daily analytics record
func updateDailyAnalytics(field string, increment float64) error {
	today := time.Now().Format("2006-01-02")
	
	// First try to update existing record
	query := fmt.Sprintf("UPDATE analytics SET %s = %s + ? WHERE date = ?", field, field)
	result, err := db.Exec(query, increment, today)
	if err != nil {
		return err
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	
	// If no record exists for today, create one
	if rowsAffected == 0 {
		query := fmt.Sprintf("INSERT INTO analytics (date, %s) VALUES (?, ?)", field)
		_, err = db.Exec(query, today, increment)
		return err
	}
	
	return nil
}

// GetTopArticles retrieves the top N articles by sentiment or fact check score
func GetTopArticles(limit int, sortBy string) ([]struct {
	Title     string
	URL       string
	Source    string
	Published time.Time
	Score     float64
}, error) {
	var orderBy string
	switch sortBy {
	case "sentiment":
		orderBy = "sentiment DESC"
	case "factcheck":
		orderBy = "fact_check_score DESC"
	default:
		orderBy = "published_at DESC"
	}
	
	query := fmt.Sprintf(`SELECT title, url, source_name, published_at, %s AS score 
		FROM articles ORDER BY %s LIMIT ?`, 
		strings.Split(sortBy, " ")[0], orderBy)
	
	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var results []struct {
		Title     string
		URL       string
		Source    string
		Published time.Time
		Score     float64
	}
	
	for rows.Next() {
		var article struct {
			Title     string
			URL       string
			Source    string
			Published time.Time
			Score     float64
		}
		
		err := rows.Scan(
			&article.Title,
			&article.URL,
			&article.Source,
			&article.Published,
			&article.Score,
		)
		
		if err != nil {
			return nil, err
		}
		
		results = append(results, article)
	}
	
	return results, nil
}
