package main

import (
	"fmt"
	"time"
)

// formatDuration returns a human-readable string for a time.Duration
func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%02dh %02dm %02ds", h, m, s)
}

// isValidCronInterval validates a cron-style interval string in minutes (e.g., "*/15 * * * *")
func isValidCronInterval(cronSpec string) bool {
	var mins int
	_, err := fmt.Sscanf(cronSpec, "*/%d * * * *", &mins)
	return err == nil && mins >= 15 && mins <= 360
}
