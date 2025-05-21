package main

import (
    "bufio"
    "encoding/json"
    "fmt"
    "io"
    "io/ioutil"
    "log"
    "os"
    "os/signal"
    "path/filepath"
    "runtime"
    "strings"
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
    NewsDigestCron    string `json:"newsDigestCron"`
    MaxPostsPerSource int    `json:"maxPostsPerSource"`
    Version           string `json:"version"`
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
    Version       string    `json:"version"`
    StartupTime   time.Time `json:"startupTime"`
    ErrorCount    int       `json:"errorCount"`
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
    logFile           *os.File
    logger            *log.Logger
)

const (
    cooldownDuration = 10 * time.Second
    configFilePath   = "config/config.json"
    sourcesFilePath  = "config/sources.yml"
    stateFilePath    = "data/state.json"
)

// === Environment and Logging Setup ===

func loadEnv() {
    // Try loading from .env file first
    if _, err := os.Stat(".env"); err == nil {
        file, err := os.Open(".env")
        if err == nil {
            defer file.Close()
            
            scanner := bufio.NewScanner(file)
            for scanner.Scan() {
                line := scanner.Text()
                // Skip comments and empty lines
                if strings.HasPrefix(line, "#") || len(strings.TrimSpace(line)) == 0 {
                    continue
                }
                
                parts := strings.SplitN(line, "=", 2)
                if len(parts) != 2 {
                    continue
                }
                
                key := strings.TrimSpace(parts[0])
                value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
                
                // Only set if not already in environment
                if os.Getenv(key) == "" {
                    os.Setenv(key, value)
                }
            }
        }
    }
}

func setupLogging() error {
    // Create logs directory if it doesn't exist
    if _, err := os.Stat("logs"); os.IsNotExist(err) {
        err = os.Mkdir("logs", 0755)
        if err != nil {
            return err
        }
    }
    
    // Open log file with date in name
    logFileName := fmt.Sprintf("logs/sankarea_%s.log", time.Now().Format("2006-01-02"))
    var err error
    logFile, err = os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
    if err != nil {
        return err
    }
    
    // Create multi-writer to log to both file and console
    multiWriter := io.MultiWriter(os.Stdout, logFile)
    logger = log.New(multiWriter, "", log.Ldate|log.Ltime)
    
    // Redirect standard logger to our logger
    log.SetOutput(multiWriter)
    log.SetFlags(log.Ldate | log.Ltime)
    
    return nil
}

func logf(format string, args ...interface{}) {
    _, file, line, _ := runtime.Caller(1)
    logger.Printf("%s:%d: %s", filepath.Base(file), line, fmt.Sprintf(format, args...))
}

func getEnvOrFail(key string) string {
    v := os.Getenv(key)
    if v == "" {
        log.Fatalf("Missing required environment variable: %s", key)
    }
    return v
}

func getEnvOrDefault(key, defaultValue string) string {
    v := os.Getenv(key)
    if v == "" {
        return defaultValue
    }
    return v
}

func fileMustExist(path string) {
    if _, err := os.Stat(path); os.IsNotExist(err) {
        log.Fatalf("ERROR: Required file not found: %s", path)
    }
}

// === File Operations ===

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

// === Logging and Error Handling ===

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
        state.ErrorCount++
        state.LastError = msg
        saveState(state)
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

// === News Functions ===

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

func postNewsDigest(dg *discordgo.Session, channelID string, sources []Source) {
    if state.Paused {
        return
    }
    
    embed := &discordgo.MessageEmbed{
        Title:       "News Digest",
        Description: "Summary of top news from various sources",
        Color:       0x0099ff,
        Timestamp:   time.Now().Format(time.RFC3339),
        Fields:      []*discordgo.MessageEmbedField{},
    }
    
    fp := gofeed.NewParser()
    for _, src := range sources {
        if !src.Active {
            continue
        }
        
        feed, err := fp.ParseURL(src.URL)
        if err != nil {
            continue
        }
        
        if len(feed.Items) > 0 {
            title := fmt.Sprintf("%s (%s)", src.Name, src.Bias)
            value := fmt.Sprintf("[%s](%s)", feed.Items[0].Title, feed.Items[0].Link)
            embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
                Name:   title,
                Value:  value,
                Inline: false,
            })
        }
    }
    
    if len(embed.Fields) > 0 {
        _, err := dg.ChannelMessageSendEmbed(channelID, embed)
        if err != nil {
            logAudit("DigestError", fmt.Sprintf("Failed to post digest: %v", err), 0xff0000)
        } else {
            state.LastDigest = time.Now()
            saveState(state)
        }
    }
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

// === Discord Command Registration ===

func registerCommands(s *discordgo.Session, appID, guildID string) error {
    commands := []*discordgo.ApplicationCommand{
        {
            Name:        "ping",
            Description: "Check if the bot is alive",
        },
        {
            Name:        "status",
            Description: "Show bot status and news posting information",
        },
        {
            Name:        "setnewsinterval",
            Description: "Set how often news is posted (in minutes)",
            Options: []*discordgo.ApplicationCommandOption{
                {
                    Type:        discordgo.ApplicationCommandOptionInteger,
                    Name:        "minutes",
                    Description: "Minutes between posts (15-360)",
                    Required:    true,
                    MinValue:    &[]float64{15}[0],
                    MaxValue:    360,
                },
            },
        },
        {
            Name:        "setdigestinterval",
            Description: "Set how often news digests are posted (in hours)",
            Options: []*discordgo.ApplicationCommandOption{
                {
                    Type:        discordgo.ApplicationCommandOptionInteger,
                    Name:        "hours",
                    Description: "Hours between digests (1-24)",
                    Required:    true,
                    MinValue:    &[]float64{1}[0],
                    MaxValue:    24,
                },
            },
            DefaultMemberPermissions: &[]int64{discordgo.PermissionAdministrator}[0],
        },
        {
            Name:        "nullshutdown",
            Description: "Shut down the bot (admin only)",
            DefaultMemberPermissions: &[]int64{discordgo.PermissionAdministrator}[0],
        },
        {
            Name:        "nullrestart",
            Description: "Restart the bot (admin only)",
            DefaultMemberPermissions: &[]int64{discordgo.PermissionAdministrator}[0],
        },
        {
            Name:        "silence",
            Description: "Timeout a user (admin only)",
            DefaultMemberPermissions: &[]int64{discordgo.PermissionModerateMembers}[0],
            Options: []*discordgo.ApplicationCommandOption{
                {
                    Type:        discordgo.ApplicationCommandOptionUser,
                    Name:        "user",
                    Description: "User to silence",
                    Required:    true,
                },
                {
                    Type:        discordgo.ApplicationCommandOptionInteger,
                    Name:        "minutes",
                    Description: "Minutes to silence for",
                    Required:    true,
                    MinValue:    &[]float64{1}[0],
                    MaxValue:    10080, // 1 week
                },
            },
        },
        {
            Name:        "unsilence",
            Description: "Remove timeout from a user (admin only)",
            DefaultMemberPermissions: &[]int64{discordgo.PermissionModerateMembers}[0],
            Options: []*discordgo.ApplicationCommandOption{
                {
                    Type:        discordgo.ApplicationCommandOptionUser,
                    Name:        "user",
                    Description: "User to unsilence",
                    Required:    true,
                },
            },
        },
        {
            Name:        "kick",
            Description: "Kick a user from the server (admin only)",
            DefaultMemberPermissions: &[]int64{discordgo.PermissionKickMembers}[0],
            Options: []*discordgo.ApplicationCommandOption{
                {
                    Type:        discordgo.ApplicationCommandOptionUser,
                    Name:        "user",
                    Description: "User to kick",
                    Required:    true,
                },
            },
        },
        {
            Name:        "ban",
            Description: "Ban a user from the server (admin only)",
            DefaultMemberPermissions: &[]int64{discordgo.PermissionBanMembers}[0],
            Options: []*discordgo.ApplicationCommandOption{
                {
                    Type:        discordgo.ApplicationCommandOptionUser,
                    Name:        "user",
                    Description: "User to ban",
                    Required:    true,
                },
            },
        },
        {
            Name:        "factcheck",
            Description: "Check if a claim is factual",
            Options: []*discordgo.ApplicationCommandOption{
                {
                    Type:        discordgo.ApplicationCommandOptionString,
                    Name:        "claim",
                    Description: "The claim to fact check",
                    Required:    true,
                },
            },
        },
        {
            Name:        "reloadconfig",
            Description: "Reload bot configuration (admin only)",
            DefaultMemberPermissions: &[]int64{discordgo.PermissionAdministrator}[0],
        },
        {
            Name:        "health",
            Description: "Check bot health status",
        },
        {
            Name:        "version",
            Description: "Show bot version information",
        },
    }

    for _, cmd := range commands {
        _, err := s.ApplicationCommandCreate(appID, guildID, cmd)
        if err != nil {
            return fmt.Errorf("failed to create '%s' command: %w", cmd.Name, err)
        }
    }
    
    return nil
}

// === Slash Command Handler ===

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

    // Prevent dangerous commands in DM
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
        
    case "setdigestinterval":
        if !isAdminOrOwner(i) {
            s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
                Type: discordgo.InteractionResponseChannelMessageWithSource,
                Data: &discordgo.InteractionResponseData{
                    Content: "Weeb, You Do Not Have The Right Privileges.",
                },
            })
            return
        }
        hours := int(i.ApplicationCommandData().Options[0].IntValue())
        if hours < 1 || hours > 24 {
            s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
                Type: discordgo.InteractionResponseChannelMessageWithSource,
                Data: &discordgo.InteractionResponseData{
                    Content: "Interval must be between 1 and 24 hours.",
                    Flags:   1 << 6,
                },
            })
            return
        }
        
        // Update digest cron
        spec := fmt.Sprintf("0 */%d * * *", hours)
        currentConfig.NewsDigestCron = spec
        saveConfig(currentConfig)
        
        // Restart cron jobs to apply new digest schedule
        if cronJob != nil {
            cronJob.Stop()
            cronJob = cron.New()
            
            // Re-add regular news job
            cronJobID, _ = cronJob.AddFunc(currentConfig.News15MinCron, func() {
                fetchAndPostNews(dg, discordChannelID, sources)
            })
            
            // Add digest job
            _, _ = cronJob.AddFunc(currentConfig.NewsDigestCron, func() {
                postNewsDigest(dg, discordChannelID, sources)
            })
            
            cronJob.Start()
        }
        
        logAudit("DigestIntervalChange", fmt.Sprintf("By <@%s>: Now every %d hours", userID, hours), 0xffcc00)
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: fmt.Sprintf("News digest interval updated to %d hours.", hours),
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
        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: "Restarting bot...",
            },
        })
        logAudit("Restart", fmt.Sprintf("Restart requested by <@%s>", userID), 0xffcc00)
        go func() {
            time.Sleep(2 * time.Second)
            os.Exit(42) // Your runner script should handle this as a restart signal
        }()

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
