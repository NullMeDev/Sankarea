package main

import (
    "fmt"
    "io/ioutil"
    "log"
    "os"
    "os/signal"
    "syscall"
    "gopkg.in/yaml.v2"
    "encoding/json"
    "github.com/bwmarrin/discordgo"
    "github.com/mmcdole/gofeed"
    "github.com/robfig/cron/v3"
)

type Source struct {
    Name string `yaml:"name"`
    URL  string `yaml:"url"`
    Bias string `yaml:"bias"`
}

type Config struct {
    News15MinCron string `json:"news15MinCron"`
}

func getEnvOrFail(key string) string {
    v := os.Getenv(key)
    if v == "" {
        log.Fatalf("Missing required environment variable: %s", key)
    }
    return v
}

func fileMustExist(path string) {
    if _, err := os.Stat(path); os.IsNotExist(err) {
        log.Fatalf("ERROR: Required file not found: %s", path)
    }
}

func loadSources() ([]Source, error) {
    b, err := ioutil.ReadFile("config/sources.yml")
    if err != nil {
        return nil, err
    }
    var sources []Source
    if err := yaml.Unmarshal(b, &sources); err != nil {
        return nil, err
    }
    return sources, nil
}

func loadConfig() (Config, error) {
    b, err := ioutil.ReadFile("config/config.json")
    if err != nil {
        return Config{}, err
    }
    var config Config
    if err := json.Unmarshal(b, &config); err != nil {
        return Config{}, err
    }
    return config, nil
}

func fetchAndPostNews(dg *discordgo.Session, channelID string, sources []Source) {
    fp := gofeed.NewParser()
    for _, src := range sources {
        feed, err := fp.ParseURL(src.URL)
        if err != nil {
            log.Printf("Failed to fetch %s: %v", src.Name, err)
            continue
        }
        if len(feed.Items) == 0 {
            continue
        }
        msg := fmt.Sprintf("**[%s] Top headline:**\n[%s](%s)", src.Name, feed.Items[0].Title, feed.Items[0].Link)
        _, err = dg.ChannelMessageSend(channelID, msg)
        if err != nil {
            log.Printf("Failed to post news for %s: %v", src.Name, err)
        }
    }
}

func main() {
    fmt.Println("Sankarea bot starting up...")

    fileMustExist("config/config.json")
    fileMustExist("config/sources.yml")
    fileMustExist("data/state.json")

    discordBotToken := getEnvOrFail("DISCORD_BOT_TOKEN")
    discordAppID := getEnvOrFail("DISCORD_APPLICATION_ID")
    discordGuildID := getEnvOrFail("DISCORD_GUILD_ID")
    discordChannelID := getEnvOrFail("DISCORD_CHANNEL_ID")

    sources, err := loadSources()
    if err != nil {
        log.Fatalf("Failed to load sources.yml: %v", err)
    }
    config, err := loadConfig()
    if err != nil {
        log.Fatalf("Failed to load config.json: %v", err)
    }

    dg, err := discordgo.New("Bot " + discordBotToken)
    if err != nil {
        log.Fatalf("Error creating Discord session: %v", err)
    }

    dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
        if i.Type == discordgo.InteractionApplicationCommand {
            switch i.ApplicationCommandData().Name {
            case "ping":
                s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
                    Type: discordgo.InteractionResponseChannelMessageWithSource,
                    Data: &discordgo.InteractionResponseData{
                        Content: "Pong!",
                    },
                })
            }
        }
    })

    err = dg.Open()
    if err != nil {
        log.Fatalf("Error opening connection to Discord: %v", err)
    }
    defer dg.Close()

    _, err = dg.ApplicationCommandCreate(discordAppID, discordGuildID, &discordgo.ApplicationCommand{
        Name:        "ping",
        Description: "Test if the bot is alive",
    })
    if err != nil {
        log.Fatalf("Cannot create slash command: %v", err)
    }

    _, err = dg.ChannelMessageSend(discordChannelID, "🟢 Sankarea bot is online and ready. Now auto-posting news every 15 minutes.")
    if err != nil {
        log.Printf("Failed to send startup message: %v", err)
    }

    // Set up the cron job for RSS news
    c := cron.New()
    _, err = c.AddFunc(config.News15MinCron, func() {
        fetchAndPostNews(dg, discordChannelID, sources)
    })
    if err != nil {
        log.Fatalf("Failed to schedule cron job: %v", err)
    }
    c.Start()

    // Run once at startup
    fetchAndPostNews(dg, discordChannelID, sources)

    fmt.Println("Sankarea bot is running. Press CTRL+C to exit.")
    stop := make(chan os.Signal, 1)
    signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
    <-stop
    fmt.Println("Sankarea bot shutting down...")
}
