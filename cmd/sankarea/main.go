package main

import (
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/bwmarrin/discordgo"
    "github.com/robfig/cron/v3"
)

var cfg *Config

// logPanic recovers and logs any panic, then exits
func logPanic() {
    if r := recover(); r != nil {
        Logger().Printf("Panic: %v", r)
        os.Exit(1)
    }
}

func main() {
    defer logPanic()
    fmt.Println("Sankarea bot starting up...")

    LoadEnv()
    FileMustExist("config/config.json")
    FileMustExist("config/sources.yml")
    EnsureDataDir()

    if err := SetupLogging(); err != nil {
        log.Printf("Warning: %v", err)
    }

    var err error
    cfg, err = LoadConfig()
    if err != nil {
        Logger().Printf("load config: %v", err)
        os.Exit(1)
    }

    sources, err := LoadSources()
    if err != nil {
        Logger().Printf("load sources: %v", err)
        os.Exit(1)
    }

    state, err := LoadState()
    if err != nil {
        Logger().Printf("load state: %v", err)
        os.Exit(1)
    }
    _ = state // you can wire state into your jobs

    dg, err := discordgo.New("Bot " + cfg.BotToken)
    if err != nil {
        Logger().Printf("create session: %v", err)
        os.Exit(1)
    }

    if err := RegisterCommands(dg, cfg.AppID, cfg.GuildID); err != nil {
        Logger().Printf("register commands: %v", err)
        os.Exit(1)
    }
    dg.AddHandler(handleInteraction)

    if err := dg.Open(); err != nil {
        Logger().Printf("open connection: %v", err)
        os.Exit(1)
    }
    Logger().Println("Sankarea bot is now running")

    // Scheduler
    c := cron.New()
    var entryID cron.EntryID
    // parse the "*/15 * * * *" style into minutes
    minutes := int(parseCron(cfg.News15MinCron).Minutes())
    UpdateCronJob(c, &entryID, minutes, dg, cfg.AuditLogChannelID, sources)
    if cfg.NewsDigestCron != "" {
        c.AddFunc(cfg.NewsDigestCron, func() {
            postNewsDigest(dg, cfg.AuditLogChannelID, sources)
        })
    }
    c.Start()

    // Wait for CTRL+C or SIGTERM
    stop := make(chan os.Signal, 1)
    signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
    <-stop

    c.Stop()
    dg.Close()
    Logger().Println("Sankarea bot shutting down...")
}
