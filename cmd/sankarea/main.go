package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/robfig/cron/v3"
)

var (
	cronJobID         cron.EntryID
	cronJob           *cron.Cron
	currentConfig     Config
	state             State
	sources           []Source
	discordChannelID  string
	auditLogChannelID string
	dg                *discordgo.Session
	discordOwnerID    string
	discordGuildID    string
	startTime         time.Time
)

func loadEnv() {
	if _, err := os.Stat(".env"); err == nil {
		file, err := os.Open(".env")
		if err == nil {
			defer file.Close()

			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "#") || len(strings.TrimSpace(line)) == 0 {
					continue
				}

				parts := strings.SplitN(line, "=", 2)
				if len(parts) != 2 {
					continue
				}

				key := strings.TrimSpace(parts[0])
				value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")

				if os.Getenv(key) == "" {
					os.Setenv(key, value)
				}
			}
		}
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	return v
}

func main() {
	defer logPanic()

	fmt.Println("Sankarea bot starting up...")

	loadEnv()

	if err := setupLogging(); err != nil {
		log.Printf("Failed to set up logging: %v", err)
	}

	startTime = time.Now()

	fileMustExist("config/config.json")
	fileMustExist("config/sources.yml")

	if _, err := os.Stat("data"); os.IsNotExist(err) {
		os.Mkdir("data", 0755)
	}

	if _, err := os.Stat("data/state.json"); os.IsNotExist(err) {
		saveState(State{
			Paused:       false,
			LastInterval: 15,
			Version:      "1.0.0",
			StartupTime:  time.Now().Format(time.RFC3339),
		})
	}

	discordBotToken := getEnvOrDefault("DISCORD_BOT_TOKEN", "")
	if discordBotToken == "" {
		log.Fatal("DISCORD_BOT_TOKEN environment variable must be set")
	}
	discordAppID := getEnvOrDefault("DISCORD_APPLICATION_ID", "")
	if discordAppID == "" {
		log.Fatal("DISCORD_APPLICATION_ID environment variable must be set")
	}
	discordGuildID = getEnvOrDefault("DISCORD_GUILD_ID", "")
	if discordGuildID == "" {
		log.Fatal("DISCORD_GUILD_ID environment variable must be set")
	}
	discordChannelID = getEnvOrDefault("DISCORD_CHANNEL_ID", "")
	if discordChannelID == "" {
		log.Fatal("DISCORD_CHANNEL_ID environment variable must be set")
	}
	discordOwnerID = getEnvOrDefault("DISCORD_OWNER_ID", "")

	var err error
	sources, err = loadSources()
	if err != nil {
		log.Fatalf("Failed to load sources.yml: %v", err)
	}

	currentConfig, err = loadConfig()
	if err != nil {
		log.Fatalf("Failed to load config.json: %v", err)
	}

	auditLogChannelID = currentConfig.AuditLogChannelID

	state, err = loadState()
	if err != nil {
		log.Printf("Failed to load state, using defaults: %v", err)
		state = State{
			Paused:      false,
			LastInterval: 15,
			Version:      "1.0.0",
			StartupTime:  time.Now().Format(time.RFC3339),
		}
	}

	dg, err = discordgo.New("Bot " + discordBotToken)
	if err != nil {
		log.Fatalf("Error creating Discord session: %v", err)
	}

	if discordOwnerID == "" {
		guild, err := dg.Guild(discordGuildID)
		if err == nil {
			discordOwnerID = guild.OwnerID
		}
	}

	dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		handleCommands(s, i)
	})

	err = dg.Open()
	if err != nil {
		log.Fatalf("Error opening connection to Discord: %v", err)
	}
	defer dg.Close()

	logf("Registering slash commands...")
	if err := registerCommands(dg, discordAppID, discordGuildID); err != nil {
		logf("Failed to register commands: %v", err)
	}

	_, err = dg.ChannelMessageSend(discordChannelID, "🟢 Sankarea bot is online and ready.")
	if err != nil {
		logf("Failed to send startup message: %v", err)
	}

	cronJob = cron.New()
	minutes := 15
	_, err = fmt.Sscanf(currentConfig.News15MinCron, "*/%d * * * *", &minutes)
	if err != nil || minutes < 15 || minutes > 360 {
		minutes = 15
	}
	updateCronJob(minutes)

	if currentConfig.NewsDigestCron != "" {
		_, err = cronJob.AddFunc(currentConfig.NewsDigestCron, func() {
			postNewsDigest(dg, discordChannelID, sources)
		})
		if err != nil {
			logf("Failed to schedule news digest: %v", err)
		}
	}

	cronJob.Start()
	fetchAndPostNews(dg, discordChannelID, sources)

	go backupScheduler()

	logf("Sankarea bot running. Press CTRL+C to exit.")
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	logf("Sankarea bot shutting down...")
}
