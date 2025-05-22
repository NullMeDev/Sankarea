package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/bwmarrin/discordgo"
)

var (
	logFile *os.File
	logger  *log.Logger
)

// setupLogging initializes logging to both console and file with date-based filenames.
func setupLogging() error {
	// Create logs directory if it doesn't exist
	if _, err := os.Stat("logs"); os.IsNotExist(err) {
		err = os.Mkdir("logs", 0755)
		if err != nil {
			return err
		}
	}

	// Open log file for appending with date
	logFileName := "logs/sankarea_" + time.Now().Format("2006-01-02") + ".log"
	var err error
	logFile, err = os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	multiWriter := io.MultiWriter(os.Stdout, logFile)
	logger = log.New(multiWriter, "", log.Ldate|log.Ltime)

	// Redirect default logger output to multiWriter as well
	log.SetOutput(multiWriter)
	log.SetFlags(log.Ldate | log.Ltime)

	return nil
}

// logf logs formatted messages with file and line number context.
func logf(format string, args ...interface{}) {
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		file = "???"
		line = 0
	}
	logger.Printf("%s:%d: %s", filepath.Base(file), line, fmt.Sprintf(format, args...))
}

// logPanic recovers from panics, logs the panic and sends a Discord alert.
func logPanic() {
	if r := recover(); r != nil {
		msg := "PANIC: " + fmt.Sprintf("%v", r)
		log.Println(msg)
		if dg != nil && auditLogChannelID != "" {
			embed := &discordgo.MessageEmbed{
				Title:       "Critical Panic",
				Description: msg,
				Color:       0xff0000,
				Timestamp:   time.Now().Format(time.RFC3339),
			}
			_, _ = dg.ChannelMessageSendEmbed(auditLogChannelID, embed)
		}
		state.ErrorCount++
		state.LastError = msg
		saveState(state)
	}
}

// logAudit sends an audit log message to the audit channel if configured.
func logAudit(action, details string, color int) {
	if auditLogChannelID == "" || dg == nil {
		return
	}
	embed := &discordgo.MessageEmbed{
		Title:       action,
		Description: details,
		Color:       color,
		Timestamp:   time.Now().Format(time.RFC3339),
	}
	_, _ = dg.ChannelMessageSendEmbed(auditLogChannelID, embed)
}
