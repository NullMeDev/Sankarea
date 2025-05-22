package main

import (
	"fmt"
	"time"
	"encoding/json"

	"github.com/bwmarrin/discordgo"
)

// ReportType defines the type of report
type ReportType int

const (
	ReportTypeWeekly ReportType = iota
	ReportTypeMonthly
	ReportTypeAudit
)

// ReportConfig defines configuration for reports
type ReportConfig struct {
	Enabled          bool     `json:"enabled"`
	RecipientUserIDs []string `json:"recipientUserIDs"`
	ChannelID        string   `json:"channelID"`
	WeeklyCron       string   `json:"weeklyCron"`   // Default: "0 9 * * 1" (Monday 9 AM)
	MonthlyCron      string   `json:"monthlyCron"`  // Default: "0 9 1 * *" (1st day of month 9 AM)
}

// ScheduleReports schedules all report generation
func ScheduleReports(s *discordgo.Session) {
	if !cfg.Reports.Enabled {
		return
	}

	// Schedule weekly report
	_, err := cronManager.AddFunc(cfg.Reports.WeeklyCron, func() {
		GenerateReport(s, ReportTypeWeekly)
	})
	if err != nil {
		HandleError("Failed to schedule weekly report", err, "reports", ErrorSeverityMedium)
	}

	// Schedule monthly report
	_, err = cronManager.AddFunc(cfg.Reports.MonthlyCron, func() {
		GenerateReport(s, ReportTypeMonthly)
	})
	if err != nil {
		HandleError("Failed to schedule monthly report", err, "reports", ErrorSeverityMedium)
	}

	Logger().Println("Report scheduling completed successfully")
}

// GenerateReport generates and sends a report
func GenerateReport(s *discordgo.Session, reportType ReportType) {
	var err error
	var report *discordgo.MessageEmbed
	var reportName string

	switch reportType {
	case ReportTypeWeekly:
		report, err = generateWeeklyReport()
		reportName = "Weekly Report"
	case ReportTypeMonthly:
		report, err = generateMonthlyReport()
		reportName = "Monthly Report"
	case ReportTypeAudit:
		report, err = generateAuditReport()
		reportName = "Audit Report"
	default:
		err = fmt.Errorf("unknown report type")
	}

	if err != nil {
		HandleError(fmt.Sprintf("Failed to generate %s", reportName), err, "reports", ErrorSeverityMedium)
		return
	}

	// Send to channel if configured
	if cfg.Reports.ChannelID != "" {
		_, err = s.ChannelMessageSendEmbed(cfg.Reports.ChannelID, report)
		if err != nil {
			HandleError(fmt.Sprintf("Failed to send %s to channel", reportName), err, "reports", ErrorSeverityMedium)
		}
	}

	// Send to all recipients via DM
	for _, userID := range cfg.Reports.RecipientUserIDs {
		dmChannel, err := s.UserChannelCreate(userID)
		if err != nil {
			HandleError(fmt.Sprintf("Failed to create DM channel for %s", userID), err, "reports", ErrorSeverityLow)
			continue
		}

		_, err = s.ChannelMessageSendEmbed(dmChannel.ID, report)
		if err != nil {
			HandleError(fmt.Sprintf("Failed to send %s to user %s", reportName, userID), err, "reports", ErrorSeverityLow)
		}
	}

	Logger().Printf("%s generated and sent successfully", reportName)
}

// generateWeeklyReport creates a weekly summary report
func generateWeeklyReport() (*discordgo.MessageEmbed, error) {
	startDate := time.Now().AddDate(0, 0, -7)
	endDate := time.Now()

	// Get statistics
	stats, err := getReportStats(startDate, endDate)
	if err != nil {
		return nil, err
	}

	// Create embed
	embed := &discordgo.MessageEmbed{
		Title:       "📊 Weekly News Report",
		Description: fmt.Sprintf("Report period: %s - %s", 
			startDate.Format("2006-01-02"), 
			endDate.Format("2006-01-02")),
		Color:      0x00AAFF,
		Timestamp:  time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Sankarea News Bot v%s", VERSION),
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Articles Posted",
				Value:  fmt.Sprintf("%d", stats.ArticlesPosted),
				Inline: true,
			},
			{
				Name:   "Categories Covered",
				Value:  fmt.Sprintf("%d", stats.Categories),
				Inline: true,
			},
			{
				Name:   "Total Sources",
				Value:  fmt.Sprintf("%d active / %d total", stats.ActiveSources, stats.TotalSources),
				Inline: true,
			},
			{
				Name:   "Fact Checks Performed",
				Value:  fmt.Sprintf("%d", stats.FactChecksPerformed),
				Inline: true,
			},
			{
				Name:   "Claims Disputed",
				Value:  fmt.Sprintf("%d", stats.ClaimsDisputed),
				Inline: true,
			},
			{
				Name:   "Content Moderation Alerts",
				Value:  fmt.Sprintf("%d", stats.ModerationAlerts),
				Inline: true,
			},
			{
				Name:   "Digests Generated",
				Value:  fmt.Sprintf("%d", stats.DigestsGenerated),
				Inline: true,
			},
			{
				Name:   "API Calls",
				Value:  fmt.Sprintf("%d", stats.APICalls),
				Inline: true,
			},
			{
				Name:   "System Uptime",
				Value:  stats.Uptime,
				Inline: true,
			},
		},
	}

	// Add top sources
	if len(stats.TopSources) > 0 {
		topSourcesValue := ""
		for i, source := range stats.TopSources {
			if i >= 5 {
				break
			}
			topSourcesValue += fmt.Sprintf("%d. **%s** (%d articles)\n", i+1, source.Name, source.Count)
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Top Sources",
			Value:  topSourcesValue,
			Inline: false,
		})
	}

	// Add trending topics
	if len(stats.TrendingTopics) > 0 {
		trendingValue := ""
		for i, topic := range stats.TrendingTopics {
			if i >= 5 {
				break
			}
			trendingValue += fmt.Sprintf("%d. **%s** (mentioned %d times)\n", i+1, topic.Topic, topic.Count)
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Trending Topics",
			Value:  trendingValue,
			Inline: false,
		})
	}

	// Add recent errors (if any)
	if stats.ErrorCount > 0 {
		errorValue := ""
		if len(stats.RecentErrors) > 0 {
			for i, err := range stats.RecentErrors {
				if i >= 3 {
					break
				}
				errorValue += fmt.Sprintf("• **%s**: %s\n", err.Component, err.Message)
			}
		} else {
			errorValue = "No detailed error information available"
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("Errors (%d total)", stats.ErrorCount),
			Value:  errorValue,
			Inline: false,
		})
	}

	return embed, nil
}

// generateMonthlyReport creates a monthly summary report (similar to weekly)
func generateMonthlyReport() (*discordgo.MessageEmbed, error) {
	// Similar to weekly report but with different date range
	startDate := time.Now().AddDate(0, -1, 0)
	endDate := time.Now()

	// Get statistics
	stats, err := getReportStats(startDate, endDate)
	if err != nil {
		return nil, err
	}

	// Create embed (similar to weekly with title changed)
	embed := &discordgo.MessageEmbed{
		Title:       "📈 Monthly News Report",
		Description: fmt.Sprintf("Report period: %s - %s", 
			startDate.Format("2006-01-02"), 
			endDate.Format("2006-01-02")),
		Color:      0x00AAFF,
		Timestamp:  time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Sankarea News Bot v%s", VERSION),
		},
	}
	
	// Add the same fields as weekly report
	// ArticlesPosted
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "Articles Posted",
		Value:  fmt.Sprintf("%d", stats.ArticlesPosted),
		Inline: true,
	})
	
	// Categories
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "Categories Covered",
		Value:  fmt.Sprintf("%d", stats.Categories),
		Inline: true,
	})
	
	// Sources
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "Total Sources",
		Value:  fmt.Sprintf("%d active / %d total", stats.ActiveSources, stats.TotalSources),
		Inline: true,
	})
	
	// Fact Checks
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "Fact Checks Performed",
		Value:  fmt.Sprintf("%d", stats.FactChecksPerformed),
		Inline: true,
	})
	
	// Claims Disputed
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "Claims Disputed",
		Value:  fmt.Sprintf("%d", stats.ClaimsDisputed),
		Inline: true,
	})
	
	// Moderation Alerts
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "Content Moderation Alerts",
		Value:  fmt.Sprintf("%d", stats.ModerationAlerts),
		Inline: true,
	})
	
	// Digests
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "Digests Generated",
		Value:  fmt.Sprintf("%d", stats.DigestsGenerated),
		Inline: true,
	})
	
	// API Calls
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "API Calls",
		Value:  fmt.Sprintf("%d", stats.APICalls),
		Inline: true,
	})
	
	// Uptime
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "System Uptime",
		Value:  stats.Uptime,
		Inline: true,
	})
	
	// Add top sources
	if len(stats.TopSources) > 0 {
		topSourcesValue := ""
		for i, source := range stats.TopSources {
			if i >= 5 {
				break
			}
			topSourcesValue += fmt.Sprintf("%d. **%s** (%d articles)\n", i+1, source.Name, source.Count)
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Top Sources",
			Value:  topSourcesValue,
			Inline: false,
		})
	}

	// Add trending topics
	if len(stats.TrendingTopics) > 0 {
		trendingValue := ""
		for i, topic := range stats.TrendingTopics {
			if i >= 5 {
				break
			}
			trendingValue += fmt.Sprintf("%d. **%s** (mentioned %d times)\n", i+1, topic.Topic, topic.Count)
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Trending Topics",
			Value:  trendingValue,
			Inline: false,
		})
	}

	// Add monthly summary paragraph
	summaryValue := "During this month, we've monitored news across multiple sources and categories. "
	
	// Add bias distribution
	if stats.LeftBiasCount > 0 || stats.RightBiasCount > 0 || stats.CenterBiasCount > 0 {
		summaryValue += fmt.Sprintf("Coverage included %d left-leaning, %d center, and %d right-leaning sources. ", 
			stats.LeftBiasCount, stats.CenterBiasCount, stats.RightBiasCount)
	}
	
	// Add most active categories
	if len(stats.TopCategories) > 0 {
		summaryValue += "The most active news categories were "
		for i, cat := range stats.TopCategories {
			if i > 0 && i == len(stats.TopCategories)-1 {
				summaryValue += " and "
			} else if i > 0 {
				summaryValue += ", "
			}
			summaryValue += cat.Name
		}
		summaryValue += ". "
	}
	
	// Add error rate if available
	if stats.ArticlesPosted > 0 && stats.ErrorCount > 0 {
		errorRate := float64(stats.ErrorCount) / float64(stats.ArticlesPosted) * 100.0
		summaryValue += fmt.Sprintf("The system operated with a %.1f%% error rate.", errorRate)
	}
	
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "Monthly Summary",
		Value:  summaryValue,
		Inline: false,
	})

	return embed, nil
}

// generateAuditReport creates an audit report focusing on system security and operations
func generateAuditReport() (*discordgo.MessageEmbed, error) {
	// Fetch audit data (this would come from a database in a real implementation)
	// For this example, we'll use sample data
	
	embed := &discordgo.MessageEmbed{
		Title:       "🔒 System Audit Report",
		Description: fmt.Sprintf("Audit report generated on %s", time.Now().Format("2006-01-02 15:04:05")),
		Color:       0xFF5500,
		Timestamp:   time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Sankarea News Bot v%s", VERSION),
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "System Status",
				Value:  "Operational",
				Inline: true,
			},
			{
				Name:   "Uptime",
				Value:  GetUptime().Round(time.Hour).String(),
				Inline: true,
			},
			{
				Name:   "API Health",
				Value:  "All APIs Operational",
				Inline: true,
			},
			{
				Name:   "Admin Actions",
				Value:  "12 (See details below)",
				Inline: true,
			},
			{
				Name:   "Moderation Actions",
				Value:  "8 (See details below)",
				Inline: true,
			},
			{
				Name:   "Security Alerts",
				Value:  "2 (Low severity)",
				Inline: true,
			},
		},
	}
	
	// Add admin actions
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "Recent Admin Actions",
		Value:  "• **2025-05-22 08:12** - Source added: 'The Guardian'\n• **2025-05-21 15:30** - User banned: Discord ID 123456789\n• **2025-05-20 09:45** - Configuration updated",
		Inline: false,
	})
	
	// Add moderation actions
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "Recent Moderation Actions",
		Value:  "• **2025-05-22 10:18** - Content filtered: High severity hate speech\n• **2025-05-19 14:22** - Content filtered: Medium severity harassment",
		Inline: false,
	})
	
	// Add security alerts
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "Security Alerts",
		Value:  "• **2025-05-20 23:14** - Multiple failed login attempts from IP 192.168.1.1\n• **2025-05-18 04:36** - API rate limit reached for OpenAI",
		Inline: false,
	})
	
	// Add recommendations
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "Recommendations",
		Value:  "1. Review moderator permissions for optimal coverage\n2. Consider increasing API rate limits for fact checking service\n3. Update security policies for improved logging",
		Inline: false,
	})

	return embed, nil
}

// ReportStats contains statistics for reports
type ReportStats struct {
	ArticlesPosted      int
	Categories          int
	ActiveSources       int
	TotalSources        int
	FactChecksPerformed int
	ClaimsDisputed      int
	ModerationAlerts    int
	DigestsGenerated    int
	APICalls            int
	ErrorCount          int
	Uptime              string
	
	// Bias distribution
	LeftBiasCount      int
	CenterBiasCount    int
	RightBiasCount     int
	
	TopSources     []SourceStat
	TopCategories  []CategoryStat
	TrendingTopics []TopicStat
	RecentErrors   []ErrorStat
}

type SourceStat struct {
	Name  string
	Count int
}

type CategoryStat struct {
	Name  string
	Count int
}

type TopicStat struct {
	Topic string
	Count int
}

type ErrorStat struct {
	Component string
	Message   string
	Time      time.Time
}

// getReportStats retrieves statistics for a report
func getReportStats(startDate, endDate time.Time) (*ReportStats, error) {
	// This would be implemented with database queries if database is available
	// For now returning sample data
	stats := &ReportStats{
		ArticlesPosted:      142,
		Categories:          5,
		ActiveSources:       8,
		TotalSources:        10,
		FactChecksPerformed: 37,
		ClaimsDisputed:      12,
		ModerationAlerts:    3,
		DigestsGenerated:    7,
		APICalls:            216,
		ErrorCount:          2,
		Uptime:              GetUptime().Round(time.Hour).String(),
		
		// Bias counts
		LeftBiasCount:      3,
		CenterBiasCount:    4, 
		RightBiasCount:     3,
		
		TopSources: []SourceStat{
			{"CNN", 32},
			{"BBC", 28},
			{"Reuters", 24},
			{"AP News", 19},
			{"Fox News", 18},
		},
		TopCategories: []CategoryStat{
			{"Politics", 48},
			{"Business", 32},
			{"World", 27},
			{"Technology", 21},
			{"Health", 14},
		},
		TrendingTopics: []TopicStat{
			{"Climate Change", 14},
			{"Economic Policy", 12},
			{"Healthcare", 9},
			{"Elections", 8},
			{"Technology", 7},
		},
		RecentErrors: []ErrorStat{
			{"RSS Fetcher", "Failed to connect to source: timeout", time.Now().Add(-24 * time.Hour)},
			{"OpenAI API", "Rate limit exceeded", time.Now().Add(-48 * time.Hour)},
		},
	}

	return stats, nil
}

// getCurrentDateFormatted returns the current date formatted for reports
func getCurrentDateFormatted() string {
	// Format is 2025-05-22 14:26:08 from user's system
	return "2025-05-22 14:26:08"
}
