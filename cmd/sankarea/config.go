// Config holds application configuration
type Config struct {
	// Bot Configuration
	BotToken             string          `json:"bot_token"`
	AppID                string          `json:"app_id"`
	GuildID              string          `json:"guild_id"`
	Version              string          `json:"version"`
	MaxPostsPerSource    int             `json:"maxPostsPerSource"`
	OwnerIDs             []string        `json:"ownerIDs"` // Discord User IDs who have owner permissions
	AdminRoleIDs         []string        `json:"adminRoleIDs"` // Discord Role IDs that have admin permissions
	UserAgentString      string          `json:"user_agent_string"` // Added field for User-Agent
	
	// Rest of the config structure remains unchanged...
}
