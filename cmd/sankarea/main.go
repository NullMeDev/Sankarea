// main.go
package main

import (
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "os"
    "os/signal"
    "strconv"
    "sync"
    "syscall"
    "time"

    "github.com/bwmarrin/discordgo"
    "github.com/mmcdole/gofeed"
    "github.com/robfig/cron/v3"
    "gopkg.in/yaml.v2"
)

type Source struct {
    Name   string `yaml:"name"`
    URL    string `yaml:"url"`
    Bias   string `yaml:"bias"`
    Active bool   `yaml:"active"`
}

type Config struct {
    News15MinCron     string `json:"news15MinCron"`
    AuditLogChannelID string `json:"auditLogChannelId"`
}

type State struct {
    Paused        bool      `json:"paused"`
    LastDigest    time.Time `json:"lastDigest"`
    LastInterval  int       `json:"lastInterval"`
    LastError     string    `json:"lastError"`
    NewsNextTime  time.Time `json:"newsNextTime"`
    FeedCount     int       `json:"feedCount"`
    Lockdown      bool      `json:"lockdown"`
    LockdownSetBy string    `json:"lockdownSetBy"`
}

var (
    cronJobID         cron.EntryID
    cronJob           *cron.Cron
    currentConfig     Config
    state             State
    sources           []Source
    sourcesLock       sync.Mutex
    discordChannelID  string
    auditLogChannelID string
    dg                *discordgo.Session
    discordOwnerID    string
    discordGuildID    string
    cooldowns         = make(map[string]time.Time)
)

const (
    cooldownDuration = 10 * time.Second
    configFilePath   = "config/config.json"
    sourcesFilePath  = "config/sources.yml"
    stateFilePath    = "data/state.json"
)

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
    b, err := ioutil.ReadFile(sourcesFilePath)
    if err != nil {
        return nil, err
    }
    var sources []Source
    if err := yaml.Unmarshal(b, &sources); err != nil {
        return nil, err
    }
    return sources, nil
}

func saveSources(sources []Source) error {
    b, err := yaml.Marshal(sources)
    if err != nil {
        return err
    }
    return ioutil.WriteFile(sourcesFilePath, b, 0644)
}

func loadConfig() (Config, error) {
    b, err := ioutil.ReadFile(configFilePath)
    if err != nil {
        return Config{}, err
    }
    var config Config
    if err := json.Unmarshal(b, &config); err != nil {
        return Config{}, err
    }
    return config, nil
}

func saveConfig(config Config) error {
    b, err := json.MarshalIndent(config, "", "  ")
    if err != nil {
        return err
    }
    return ioutil.WriteFile(configFilePath, b, 0644)
}

func loadState() (State, error) {
    b, err := ioutil.ReadFile(stateFilePath)
    if err != nil {
        return State{}, err
    }
    var state State
    if err := json.Unmarshal(b, &state); err != nil {
        return State{}, err
    }
    return state, nil
}

func saveState(state State) error {
    b, err := json.MarshalIndent(state, "", "  ")
    if err != nil {
        return err
    }
    return ioutil.WriteFile(stateFilePath, b, 0644)
}

func logAudit(action, details string, color int) {
    if auditLogChannelID == "" {
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

func isAdminOrOwner(i *discordgo.InteractionCreate) bool {
    if i.GuildID != "" && discordOwnerID != "" && i.Member.User.ID == discordOwnerID {
        return true
    }
    const adminPerm = 0x00000008
    if i.Member.Permissions&adminPerm == adminPerm {
        return true
    }
    return false
}

func canTarget(i *discordgo.InteractionCreate, targetID string) bool {
    if targetID == discordOwnerID {
        return false
    }
    userRoles := i.Member.Roles
    member, err := dg.GuildMember(i.GuildID, targetID)
    if err != nil {
        return false
    }
    for _, rid := range member.Roles {
        for _, myrid := range userRoles {
            if rid == myrid {
                return false
            }
        }
    }
    return true
}

func enforceCooldown(userID, command string) bool {
    k := userID + "|" + command
    last, ok := cooldowns[k]
    if ok && time.Since(last) < cooldownDuration {
        return false
    }
    cooldowns[k] = time.Now()
    return true
}

func fetchAndPostNews(dg *discordgo.Session, channelID string, sources []Source) {
    sourcesLock.Lock()
    defer sourcesLock.Unlock()
    if state.Paused {
        return
    }
    fp := gofeed.NewParser()
    posted := 0
    for _, src := range sources {
        if !src.Active {
            continue
        }
        feed, err := fp.ParseURL(src.URL)
        if err != nil {
            logAudit("FeedError", fmt.Sprintf("Failed to fetch %s: %v", src.Name, err), 0xff0000)
            continue
        }
        if len(feed.Items) == 0 {
            continue
        }
        msg := fmt.Sprintf("**[%s]** *(bias: %s)*
[%s](%s)", src.Name, src.Bias, feed.Items[0].Title, feed.Items[0].Link)
        _, err = dg.ChannelMessageSend(channelID, msg)
        if err != nil {
            logAudit("PostError", fmt.Sprintf("Failed to post %s: %v", src.Name, err), 0xff0000)
        } else {
            posted++
        }
    }
    state.NewsNextTime = time.Now().Add(parseCron(currentConfig.News15MinCron))
    state.FeedCount = posted
    saveState(state)
}

func parseCron(cronSpec string) time.Duration {
    var mins int
    _, err := fmt.Sscanf(cronSpec, "*/%d * * * *", &mins)
    if err != nil || mins < 15 {
        return 15 * time.Minute
    }
    return time.Duration(mins) * time.Minute
}

func updateCronJob(minutes int) {
    if cronJob != nil && cronJobID != 0 {
        cronJob.Remove(cronJobID)
    }
    spec := fmt.Sprintf("*/%d * * * *", minutes)
    id, err := cronJob.AddFunc(spec, func() {
        fetchAndPostNews(dg, discordChannelID, sources)
    })
    if err != nil {
        logAudit("CronError", fmt.Sprintf("Failed to update cron job: %v", err), 0xff0000)
        return
    }
    cronJobID = id
    currentConfig.News15MinCron = spec
    state.LastInterval = minutes
    saveConfig(currentConfig)
    saveState(state)
}

func main() {
    fmt.Println("Sankarea bot starting up...")

    fileMustExist("config/config.json")
    fileMustExist("config/sources.yml")
    fileMustExist("data/state.json")

    discordBotToken := getEnvOrFail("DISCORD_BOT_TOKEN")
    discordAppID := getEnvOrFail("DISCORD_APPLICATION_ID")
    discordGuildID = getEnvOrFail("DISCORD_GUILD_ID")
    discordChannelID = getEnvOrFail("DISCORD_CHANNEL_ID")

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

    state, _ = loadState()

    dg, err = discordgo.New("Bot " + discordBotToken)
    if err != nil {
        log.Fatalf("Error creating Discord session: %v", err)
    }

    guild, err := dg.Guild(discordGuildID)
    if err == nil {
        discordOwnerID = guild.OwnerID
    } else {
        discordOwnerID = ""
    }

    // HANDLER OMITTED HERE FOR SIZE - you can add the rest of the commands as described above.

    err = dg.Open()
    if err != nil {
        log.Fatalf("Error opening connection to Discord: %v", err)
    }
    defer dg.Close()

    _, err = dg.ChannelMessageSend(discordChannelID, "🟢 Sankarea bot is online and ready. Use /setnewsinterval to control posting frequency.")
    if err != nil {
        log.Printf("Failed to send startup message: %v", err)
    }

    cronJob = cron.New()
    var minutes int
    _, err = fmt.Sscanf(currentConfig.News15MinCron, "*/%d * * * *", &minutes)
    if err != nil || minutes < 15 || minutes > 360 {
        minutes = 15
    }
    updateCronJob(minutes)
    cronJob.Start()
    fetchAndPostNews(dg, discordChannelID, sources)

    fmt.Println("Sankarea bot is running. Press CTRL+C to exit.")
    stop := make(chan os.Signal, 1)
    signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
    <-stop
    fmt.Println("Sankarea bot shutting down...")
}
