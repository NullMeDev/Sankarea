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

// Panic/critical error logger (to audit channel if possible)
func logPanic() {
    if r := recover(); r != nil {
        msg := fmt.Sprintf("PANIC: %v", r)
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
    }
}

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

// === News ===
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
        msg := fmt.Sprintf("**[%s]** *(bias: %s)*\n[%s](%s)", src.Name, src.Bias, feed.Items[0].Title, feed.Items[0].Link)
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
// === Moderation/Admin Checks ===
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

// Role-aware: can't mod equal/higher
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

// Rate limiting per user+command
func enforceCooldown(userID, command string) bool {
    k := userID + "|" + command
    last, ok := cooldowns[k]
    if ok && time.Since(last) < cooldownDuration {
        return false
    }
    cooldowns[k] = time.Now()
    return true
}

// === Fact Check Command Stub ===
func factCheck(claim string) string {
    // Placeholder: integrate with your API (Google/ClaimBuster)
    // See TODOs for extension
    return fmt.Sprintf("Fact-check for '%s':\n[TODO: Integrate fact-check APIs.]", claim)
}
// === Slash Command Handler (core pattern) ===
func handleCommands(s *discordgo.Session, i *discordgo.InteractionCreate) {
    defer logPanic()
    name := i.ApplicationCommandData().Name
    userID := i.Member.User.ID

    if !enforceCooldown(userID, name) {
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: "Slow down. Try again in a moment.",
                Flags:   1 << 6,
            },
        })
        return
    }

    switch name {
    case "ping":
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: "Pong!",
            },
        })
    case "status", "uptime":
        paused := "No"
        if state.Paused {
            paused = "Yes"
        }
        summary := fmt.Sprintf("News posting paused: **%s**\nFeeds enabled: **%d**\nCurrent interval: **%d minutes**\nNext post: **%s**\nLockdown: **%v**\nUptime: **%s**",
            paused, state.FeedCount, state.LastInterval, state.NewsNextTime.Format(time.RFC1123), state.Lockdown, getUptime())
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: summary,
            },
        })
    case "setnewsinterval":
        if !isAdminOrOwner(i) {
            s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
                Type: discordgo.InteractionResponseChannelMessageWithSource,
                Data: &discordgo.InteractionResponseData{
                    Content: "Weeb, You Do Not Have The Right Privileges.",
                },
            })
            return
        }
        mins := int(i.ApplicationCommandData().Options[0].IntValue())
        if mins < 15 || mins > 360 {
            s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
                Type: discordgo.InteractionResponseChannelMessageWithSource,
                Data: &discordgo.InteractionResponseData{
                    Content: "Interval must be between 15 and 360 minutes.",
                    Flags:   1 << 6,
                },
            })
            return
        }
        updateCronJob(mins)
        logAudit("IntervalChange", fmt.Sprintf("By <@%s>: Now every %d min", userID, mins), 0xffcc00)
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: fmt.Sprintf("News interval updated to %d minutes.", mins),
            },
        })
    case "nullshutdown":
        if !isAdminOrOwner(i) {
            s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
                Type: discordgo.InteractionResponseChannelMessageWithSource,
                Data: &discordgo.InteractionResponseData{
                    Content: "Weeb, You Do Not Have The Right Privileges.",
                },
            })
            return
        }
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: "Shutting down bot. Goodbye.",
            },
        })
        logAudit("Shutdown", fmt.Sprintf("Shutdown requested by <@%s>", userID), 0xff0000)
        os.Exit(0)
    case "nullrestart":
        if !isAdminOrOwner(i) {
            s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
                Type: discordgo.InteractionResponseChannelMessageWithSource,
                Data: &discordgo.InteractionResponseData{
                    Content: "Weeb, You Do Not Have The Right Privileges.",
                },
            })
            return
        }
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: "Restarting bot...",
            },
        })
        logAudit("Restart", fmt.Sprintf("Restart requested by <@%s>", userID), 0xffcc00)
        os.Exit(42) // Use a runner script to handle 42 as a restart

    case "version":
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: "Sankarea Bot Version: 1.0.0", // Update as needed
            },
        })
    case "reloadconfig":
        if !isAdminOrOwner(i) {
            s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
                Type: discordgo.InteractionResponseChannelMessageWithSource,
                Data: &discordgo.InteractionResponseData{
                    Content: "Weeb, You Do Not Have The Right Privileges.",
                },
            })
            return
        }
        newConfig, err := loadConfig()
        if err != nil {
            logAudit("ReloadConfigError", err.Error(), 0xff0000)
            s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
                Type: discordgo.InteractionResponseChannelMessageWithSource,
                Data: &discordgo.InteractionResponseData{
                    Content: "Error reloading config.",
                },
            })
            return
        }
        currentConfig = newConfig
        logAudit("ReloadConfig", fmt.Sprintf("Reloaded config by <@%s>", userID), 0x00ff00)
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: "Config reloaded.",
            },
        })
    case "health":
        health := checkHealth()
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: health,
            },
        })
    // TODO: Add your other moderation and feed management commands here.
    }
}

// Uptime/Health Helpers
var startTime = time.Now()

func getUptime() string {
    dur := time.Since(startTime)
    return dur.Truncate(time.Second).String()
}

// Health status check (can expand)
func checkHealth() string {
    // Ping Discord, check APIs, report last error
    return fmt.Sprintf("Bot is healthy. Last error: %s", state.LastError)
}

// === Main with Panic Protection and Startup Logging ===
func main() {
    defer logPanic()
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

    dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
        handleCommands(s, i)
    })

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

    go backupScheduler()

    fmt.Println("Sankarea bot is running. Press CTRL+C to exit.")
    stop := make(chan os.Signal, 1)
    signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
    <-stop
    fmt.Println("Sankarea bot shutting down...")
}

// === Scheduled Backup Routine ===
func backupScheduler() {
    for {
        now := time.Now()
        backupName := fmt.Sprintf("backup_%s.json", now.Format("20060102_150405"))
        ioutil.WriteFile("data/"+backupName, []byte(fmt.Sprintf("Config: %+v\nState: %+v", currentConfig, state)), 0644)
        // Keep only last 7 backups
        files, _ := ioutil.ReadDir("data")
        var backups []os.FileInfo
        for _, f := range files {
            if len(f.Name()) > 7 && f.Name()[:7] == "backup_" {
                backups = append(backups, f)
            }
        }
        if len(backups) > 7 {
            for _, old := range backups[:len(backups)-7] {
                os.Remove("data/" + old.Name())
            }
        }
        time.Sleep(24 * time.Hour)
    }
}

// === TODOs and Future Extensions ===
// TODO: Implement all missing admin/moderation commands (/kick, /ban, user picker, etc)
// TODO: Integrate ClaimBuster/Google Fact Check API
// TODO: Improve health check to actually ping each integrated API
// TODO: Add configurable roles/permissions for custom admin levels
// TODO: Support for dynamic news feeds in real time
// === Slash Command Handler (core pattern) ===
func handleCommands(s *discordgo.Session, i *discordgo.InteractionCreate) {
    defer logPanic()
    name := i.ApplicationCommandData().Name
    userID := i.Member.User.ID

    if !enforceCooldown(userID, name) {
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: "Slow down. Try again in a moment.",
                Flags:   1 << 6,
            },
        })
        return
    }

    // Only allow dangerous commands in guild, not DM
    if i.GuildID == "" && (name == "kick" || name == "ban" || name == "nullrestart" || name == "nullshutdown" || name == "setnewsinterval" || name == "lockdown" || name == "unlock" || name == "silence" || name == "unsilence") {
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: "This command cannot be used in DM.",
                Flags:   1 << 6,
            },
        })
        return
    }

    switch name {
    case "ping":
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: "Pong!",
            },
        })

    case "status":
        paused := "No"
        if state.Paused {
            paused = "Yes"
        }
        summary := fmt.Sprintf("News paused: **%s**\nFeeds: **%d**\nInterval: **%d min**\nNext post: **%s**\nLockdown: **%v**", paused, state.FeedCount, state.LastInterval, state.NewsNextTime.Format(time.RFC1123), state.Lockdown)
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: summary,
            },
        })

    case "setnewsinterval":
        if !isAdminOrOwner(i) {
            s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
                Type: discordgo.InteractionResponseChannelMessageWithSource,
                Data: &discordgo.InteractionResponseData{
                    Content: "Weeb, You Do Not Have The Right Privileges.",
                },
            })
            return
        }
        mins := int(i.ApplicationCommandData().Options[0].IntValue())
        if mins < 15 || mins > 360 {
            s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
                Type: discordgo.InteractionResponseChannelMessageWithSource,
                Data: &discordgo.InteractionResponseData{
                    Content: "Interval must be between 15 and 360 minutes.",
                    Flags:   1 << 6,
                },
            })
            return
        }
        updateCronJob(mins)
        logAudit("IntervalChange", fmt.Sprintf("By <@%s>: Now every %d min", userID, mins), 0xffcc00)
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: fmt.Sprintf("News interval updated to %d minutes.", mins),
            },
        })

    // === Moderation/Admin Commands: Silence, Unsilence, Kick, Ban ===
    case "silence":
        if !isAdminOrOwner(i) {
            s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
                Type: discordgo.InteractionResponseChannelMessageWithSource,
                Data: &discordgo.InteractionResponseData{
                    Content: "Weeb, You Do Not Have The Right Privileges.",
                },
            })
            return
        }
        targetUser := i.ApplicationCommandData().Options[0].UserValue(s)
        mins := int(i.ApplicationCommandData().Options[1].IntValue())
        if !canTarget(i, targetUser.ID) {
            s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
                Type: discordgo.InteractionResponseChannelMessageWithSource,
                Data: &discordgo.InteractionResponseData{
                    Content: "Cannot silence a user with equal/higher permissions.",
                },
            })
            return
        }
        until := time.Now().Add(time.Duration(mins) * time.Minute)
        err := s.GuildMemberTimeout(i.GuildID, targetUser.ID, &until)
        if err != nil {
            s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
                Type: discordgo.InteractionResponseChannelMessageWithSource,
                Data: &discordgo.InteractionResponseData{
                    Content: "Failed to silence user: " + err.Error(),
                },
            })
            logAudit("SilenceFail", fmt.Sprintf("Attempt on <@%s> by <@%s>: %v", targetUser.ID, userID, err), 0xff0000)
            return
        }
        logAudit("Silenced", fmt.Sprintf("<@%s> silenced for %d min by <@%s>", targetUser.ID, mins, userID), 0xffcc00)
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: fmt.Sprintf("User <@%s> silenced for %d minutes.", targetUser.ID, mins),
            },
        })

    case "unsilence":
        if !isAdminOrOwner(i) {
            s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
                Type: discordgo.InteractionResponseChannelMessageWithSource,
                Data: &discordgo.InteractionResponseData{
                    Content: "Weeb, You Do Not Have The Right Privileges.",
                },
            })
            return
        }
        targetUser := i.ApplicationCommandData().Options[0].UserValue(s)
        err := s.GuildMemberTimeout(i.GuildID, targetUser.ID, nil)
        if err != nil {
            s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
                Type: discordgo.InteractionResponseChannelMessageWithSource,
                Data: &discordgo.InteractionResponseData{
                    Content: "Failed to unsilence user: " + err.Error(),
                },
            })
            logAudit("UnsilenceFail", fmt.Sprintf("Attempt on <@%s> by <@%s>: %v", targetUser.ID, userID, err), 0xff0000)
            return
        }
        logAudit("Unsilenced", fmt.Sprintf("<@%s> unsilenced by <@%s>", targetUser.ID, userID), 0x00ff00)
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: fmt.Sprintf("User <@%s> unsilenced.", targetUser.ID),
            },
        })

    case "kick":
        if !isAdminOrOwner(i) {
            s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
                Type: discordgo.InteractionResponseChannelMessageWithSource,
                Data: &discordgo.InteractionResponseData{
                    Content: "Weeb, You Do Not Have The Right Privileges.",
                },
            })
            return
        }
        targetUser := i.ApplicationCommandData().Options[0].UserValue(s)
        if !canTarget(i, targetUser.ID) {
            s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
                Type: discordgo.InteractionResponseChannelMessageWithSource,
                Data: &discordgo.InteractionResponseData{
                    Content: "Cannot kick user with equal/higher permissions.",
                },
            })
            return
        }
        err := s.GuildMemberDeleteWithReason(i.GuildID, targetUser.ID, "Kicked by admin/owner via Sankarea bot")
        if err != nil {
            s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
                Type: discordgo.InteractionResponseChannelMessageWithSource,
                Data: &discordgo.InteractionResponseData{
                    Content: "Failed to kick user: " + err.Error(),
                },
            })
            logAudit("KickFail", fmt.Sprintf("Attempt on <@%s> by <@%s>: %v", targetUser.ID, userID, err), 0xff0000)
            return
        }
        logAudit("Kicked", fmt.Sprintf("<@%s> kicked by <@%s>", targetUser.ID, userID), 0xff6600)
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: fmt.Sprintf("User <@%s> kicked.", targetUser.ID),
            },
        })

    case "ban":
        if !isAdminOrOwner(i) {
            s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
                Type: discordgo.InteractionResponseChannelMessageWithSource,
                Data: &discordgo.InteractionResponseData{
                    Content: "Weeb, You Do Not Have The Right Privileges.",
                },
            })
            return
        }
        targetUser := i.ApplicationCommandData().Options[0].UserValue(s)
        if !canTarget(i, targetUser.ID) {
            s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
                Type: discordgo.InteractionResponseChannelMessageWithSource,
                Data: &discordgo.InteractionResponseData{
                    Content: "Cannot ban user with equal/higher permissions.",
                },
            })
            return
        }
        err := s.GuildBanCreateWithReason(i.GuildID, targetUser.ID, "Banned by admin/owner via Sankarea bot", 0)
        if err != nil {
            s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
                Type: discordgo.InteractionResponseChannelMessageWithSource,
                Data: &discordgo.InteractionResponseData{
                    Content: "Failed to ban user: " + err.Error(),
                },
            })
            logAudit("BanFail", fmt.Sprintf("Attempt on <@%s> by <@%s>: %v", targetUser.ID, userID, err), 0xff0000)
            return
        }
        logAudit("Banned", fmt.Sprintf("<@%s> banned by <@%s>", targetUser.ID, userID), 0xff0000)
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: fmt.Sprintf("User <@%s> banned.", targetUser.ID),
            },
        })

    // === Fact Check Command ===
    case "factcheck":
        claim := i.ApplicationCommandData().Options[0].StringValue()
        resp := factCheck(claim)
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: resp,
            },
        })

    // === Reload Config ===
    case "reloadconfig":
        if !isAdminOrOwner(i) {
            s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
                Type: discordgo.InteractionResponseChannelMessageWithSource,
                Data: &discordgo.InteractionResponseData{
                    Content: "Weeb, You Do Not Have The Right Privileges.",
                },
            })
            return
        }
        newCfg, err := loadConfig()
        if err != nil {
            s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
                Type: discordgo.InteractionResponseChannelMessageWithSource,
                Data: &discordgo.InteractionResponseData{
                    Content: "Config reload failed: " + err.Error(),
                },
            })
            return
        }
        currentConfig = newCfg
        auditLogChannelID = currentConfig.AuditLogChannelID
        sources, _ = loadSources()
        logAudit("ConfigReload", fmt.Sprintf("By <@%s>", userID), 0x0099ff)
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: "Config reloaded successfully.",
            },
        })

    // === Health Check ===
    case "health":
        content := fmt.Sprintf("Bot running. Time: %s\nFeeds: %d\nInterval: %d min\nPaused: %v\nGoRoutines: %d",
            time.Now().Format(time.RFC1123), len(sources), state.LastInterval, state.Paused, sync.NumGoroutine())
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: content,
            },
        })

    // === Uptime ===
    case "uptime":
        uptime := time.Since(state.NewsNextTime.Add(-parseCron(currentConfig.News15MinCron)))
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: fmt.Sprintf("Uptime: %s", uptime),
            },
        })

    // === Version ===
    case "version":
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: "Sankarea bot v1.0.0 (custom, latest).",
            },
        })

    // === Null Restart & Shutdown ===
    case "nullshutdown":
        if !isAdminOrOwner(i) {
            s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
                Type: discordgo.InteractionResponseChannelMessageWithSource,
                Data: &discordgo.InteractionResponseData{
                    Content: "Weeb, You Do Not Have The Right Privileges.",
                },
            })
            return
        }
        logAudit("Shutdown", fmt.Sprintf("By <@%s>", userID), 0x888888)
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: "Bot is shutting down.",
            },
        })
        go func() {
            time.Sleep(2 * time.Second)
            os.Exit(0)
        }()

    case "nullrestart":
        if !isAdminOrOwner(i) {
            s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
                Type: discordgo.InteractionResponseChannelMessageWithSource,
                Data: &discordgo.InteractionResponseData{
                    Content: "Weeb, You Do Not Have The Right Privileges.",
                },
            })
            return
        }
        logAudit("Restart", fmt.Sprintf("By <@%s>", userID), 0x8888ff)
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: "Bot is restarting.",
            },
        })
        go func() {
            time.Sleep(2 * time.Second)
            os.Exit(42) // Special code for your run loop
        }()

    // === Add other admin/news/feed/backup/lockdown commands here as needed ===

    default:
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: "Unknown or unimplemented command.",
            },
        })
    }
}
// === Uptime Helper ===
var startTime = time.Now()

func getUptime() string {
    return time.Since(startTime).Truncate(time.Second).String()
}

// Health status check (can expand)
func checkHealth() string {
    return fmt.Sprintf("Bot is healthy. Last error: %s", state.LastError)
}

// === Scheduled Backup Routine ===
func backupScheduler() {
    for {
        now := time.Now()
        backupName := fmt.Sprintf("backup_%s.json", now.Format("20060102_150405"))
        ioutil.WriteFile("data/"+backupName, []byte(fmt.Sprintf("Config: %+v\nState: %+v", currentConfig, state)), 0644)
        // Keep only last 7 backups
        files, _ := ioutil.ReadDir("data")
        var backups []os.FileInfo
        for _, f := range files {
            if len(f.Name()) > 7 && f.Name()[:7] == "backup_" {
                backups = append(backups, f)
            }
        }
        if len(backups) > 7 {
            for _, old := range backups[:len(backups)-7] {
                os.Remove("data/" + old.Name())
            }
        }
        time.Sleep(24 * time.Hour)
    }
}

// === Main ===
func main() {
    defer logPanic()
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

    dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
        handleCommands(s, i)
    })

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

    go backupScheduler()

    fmt.Println("Sankarea bot is running. Press CTRL+C to exit.")
    stop := make(chan os.Signal, 1)
    signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
    <-stop
    fmt.Println("Sankarea bot shutting down...")
}

// === TODOs and Future Extensions ===
// TODO: Expand admin/mod commands, improve error reporting, support dynamic feeds, etc.
// TODO: Integrate fact-checking APIs for real-time validation.
// TODO: Add more user/admin feedback and utility features as desired.
