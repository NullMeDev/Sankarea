// cmd/sankarea/constants.go
package main

import "time"

// Application constants
const (
    // Application information
    AppName    = "Sankarea"
    AppVersion = "1.0.0"
    AppAuthor  = "NullMeDev"
    AppRepo    = "https://github.com/NullMeDev/sankarea"

    // Default configuration
    DefaultConfigPath = "config.json"
    DefaultDataDir   = "data"
    DefaultLogDir    = "logs"
    DefaultDBName    = "sankarea"

    // Time-related constants
    DefaultTimeout      = 30 * time.Second
    DefaultRetryDelay   = 5 * time.Second
    MaxRetryDelay       = 5 * time.Minute
    HeartbeatInterval   = 15 * time.Second
    MetricsInterval     = 1 * time.Minute
    StateBackupInterval = 1 * time.Hour
    
    // Rate limits
    MaxRequestsPerMinute = 60
    MaxNewsUpdatesPerHour = 30
    MaxDigestsPerDay = 24

    // Cache settings
    DefaultCacheDuration = 5 * time.Minute
    MaxCacheSize        = 1000
    MaxCacheAge         = 24 * time.Hour

    // Discord-related constants
    MaxMessageLength    = 2000
    MaxEmbedFields     = 25
    MaxEmbedLength     = 4096
    DefaultPrefix      = "!"
    
    // API-related constants
    DefaultAPIPort     = 8080
    MaxPayloadSize     = 1024 * 1024 // 1MB
    DefaultPageSize    = 20
    MaxPageSize        = 100
    
    // Database-related constants
    MaxConnections     = 20
    ConnectionTimeout  = 10 * time.Second
    QueryTimeout       = 30 * time.Second
    MaxQueryRetries    = 3
)

// Command categories
const (
    CategoryGeneral   = "general"
    CategoryNews      = "news"
    CategoryAdmin     = "admin"
    CategorySettings  = "settings"
    CategoryUtility   = "utility"
)

// Permission levels
const (
    PermissionUser  = iota
    PermissionMod
    PermissionAdmin
    PermissionOwner
)

// Status codes
const (
    StatusOK       = "ok"
    StatusDegraded = "degraded"
    StatusError    = "error"
    StatusStarting = "starting"
)

// Event types
const (
    EventNews     = "news"
    EventDigest   = "digest"
    EventCommand  = "command"
    EventError    = "error"
    EventMetrics  = "metrics"
    EventBackup   = "backup"
)

// News categories
const (
    CategoryTechnology = "technology"
    CategoryBusiness  = "business"
    CategoryScience   = "science"
    CategoryHealth    = "health"
    CategoryPolitics  = "politics"
    CategorySports    = "sports"
    CategoryWorld     = "world"
)

// Cache keys
const (
    CacheKeyConfig     = "config"
    CacheKeyGuilds     = "guilds"
    CacheKeyNews       = "news"
    CacheKeyDigest     = "digest"
    CacheKeyMetrics    = "metrics"
    CacheKeyCommands   = "commands"
)

// Environment variables
const (
    EnvToken          = "SANKAREA_TOKEN"
    EnvConfigPath     = "SANKAREA_CONFIG"
    EnvLogLevel       = "SANKAREA_LOG_LEVEL"
    EnvEnvironment    = "SANKAREA_ENV"
    EnvDatabaseURL    = "SANKAREA_DB_URL"
    EnvAPIPort        = "SANKAREA_API_PORT"
    EnvNewsAPIKey     = "SANKAREA_NEWS_API_KEY"
)

// File paths
const (
    PathConfig        = "config.json"
    PathState         = "data/state.json"
    PathLogs          = "logs/sankarea.log"
    PathErrorLogs     = "logs/error.log"
    PathAccessLogs    = "logs/access.log"
    PathMetricsDB     = "data/metrics.db"
)

// API endpoints
const (
    APIPathHealth     = "/health"
    APIPathMetrics    = "/metrics"
    APIPathNews       = "/news"
    APIPathDigest     = "/digest"
    APIPathGuilds     = "/guilds"
    APIPathCommands   = "/commands"
    APIPathConfig     = "/config"
)

// HTTP headers
const (
    HeaderAuth        = "X-Sankarea-Auth"
    HeaderRequestID   = "X-Request-ID"
    HeaderRateLimit   = "X-Rate-Limit"
    HeaderRateRemain  = "X-Rate-Remaining"
    HeaderRateReset   = "X-Rate-Reset"
)

// Error messages
const (
    ErrMsgConfigLoad  = "failed to load configuration"
    ErrMsgDBConnect   = "failed to connect to database"
    ErrMsgAuthFailed  = "authentication failed"
    ErrMsgRateLimit   = "rate limit exceeded"
    ErrMsgInvalidJSON = "invalid JSON payload"
    ErrMsgNotFound    = "resource not found"
    ErrMsgInternal    = "internal server error"
)

// Features (for feature flags)
const (
    FeatureNewDigest  = "new_digest_format"
    FeatureAPIv2      = "api_v2"
    FeatureMetrics    = "enhanced_metrics"
    FeatureWebhooks   = "webhooks"
    FeatureAnalytics  = "analytics"
)
