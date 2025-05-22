package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
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

	discordBotToken := getEnvOrFail("DISCORD_BOT_TOKEN")
	discordAppID := getEnvOrFail("DISCORD_APPLICATION_ID")
	discordGuildID = getEnvOrFail("DISCORD_GUILD_ID")
	discordChannelID = getEnvOrFail("DISCORD_CHANNEL_ID")
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
