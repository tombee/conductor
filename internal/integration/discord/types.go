package discord

import "time"

// Message represents a Discord message.
type Message struct {
	ID              string     `json:"id"`
	ChannelID       string     `json:"channel_id"`
	Author          User       `json:"author"`
	Content         string     `json:"content"`
	Timestamp       time.Time  `json:"timestamp"`
	EditedTimestamp *time.Time `json:"edited_timestamp,omitempty"`
	TTS             bool       `json:"tts"`
	MentionEveryone bool       `json:"mention_everyone"`
	Mentions        []User     `json:"mentions"`
	Embeds          []Embed    `json:"embeds"`
	Reactions       []Reaction `json:"reactions,omitempty"`
	Pinned          bool       `json:"pinned"`
	Type            int        `json:"type"`
}

// User represents a Discord user.
type User struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator"`
	Avatar        string `json:"avatar,omitempty"`
	Bot           bool   `json:"bot,omitempty"`
}

// Channel represents a Discord channel.
type Channel struct {
	ID                   string                `json:"id"`
	Type                 int                   `json:"type"`
	GuildID              string                `json:"guild_id,omitempty"`
	Position             int                   `json:"position,omitempty"`
	Name                 string                `json:"name,omitempty"`
	Topic                string                `json:"topic,omitempty"`
	NSFW                 bool                  `json:"nsfw,omitempty"`
	LastMessageID        string                `json:"last_message_id,omitempty"`
	ParentID             string                `json:"parent_id,omitempty"`
	PermissionOverwrites []PermissionOverwrite `json:"permission_overwrites,omitempty"`
}

// PermissionOverwrite represents permission overwrites for a role or user in a channel.
type PermissionOverwrite struct {
	ID    string `json:"id"`
	Type  int    `json:"type"`
	Allow string `json:"allow"`
	Deny  string `json:"deny"`
}

// Member represents a Discord guild member.
type Member struct {
	User                       *User      `json:"user,omitempty"`
	Nick                       string     `json:"nick,omitempty"`
	Avatar                     string     `json:"avatar,omitempty"`
	Roles                      []string   `json:"roles"`
	JoinedAt                   time.Time  `json:"joined_at"`
	PremiumSince               *time.Time `json:"premium_since,omitempty"`
	Deaf                       bool       `json:"deaf"`
	Mute                       bool       `json:"mute"`
	Pending                    bool       `json:"pending,omitempty"`
	CommunicationDisabledUntil *time.Time `json:"communication_disabled_until,omitempty"`
}

// Embed represents a Discord message embed.
type Embed struct {
	Title       string       `json:"title,omitempty"`
	Description string       `json:"description,omitempty"`
	URL         string       `json:"url,omitempty"`
	Timestamp   *time.Time   `json:"timestamp,omitempty"`
	Color       int          `json:"color,omitempty"`
	Footer      *EmbedFooter `json:"footer,omitempty"`
	Image       *EmbedImage  `json:"image,omitempty"`
	Thumbnail   *EmbedImage  `json:"thumbnail,omitempty"`
	Author      *EmbedAuthor `json:"author,omitempty"`
	Fields      []EmbedField `json:"fields,omitempty"`
}

// EmbedFooter represents the footer of an embed.
type EmbedFooter struct {
	Text    string `json:"text"`
	IconURL string `json:"icon_url,omitempty"`
}

// EmbedImage represents an image in an embed.
type EmbedImage struct {
	URL      string `json:"url"`
	ProxyURL string `json:"proxy_url,omitempty"`
	Height   int    `json:"height,omitempty"`
	Width    int    `json:"width,omitempty"`
}

// EmbedAuthor represents the author of an embed.
type EmbedAuthor struct {
	Name    string `json:"name"`
	URL     string `json:"url,omitempty"`
	IconURL string `json:"icon_url,omitempty"`
}

// EmbedField represents a field in an embed.
type EmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// Reaction represents a reaction on a message.
type Reaction struct {
	Count int   `json:"count"`
	Me    bool  `json:"me"`
	Emoji Emoji `json:"emoji"`
}

// Emoji represents a Discord emoji.
type Emoji struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name"`
	Animated bool   `json:"animated,omitempty"`
}

// Webhook represents a Discord webhook.
type Webhook struct {
	ID            string `json:"id"`
	Type          int    `json:"type"`
	GuildID       string `json:"guild_id,omitempty"`
	ChannelID     string `json:"channel_id"`
	User          *User  `json:"user,omitempty"`
	Name          string `json:"name,omitempty"`
	Avatar        string `json:"avatar,omitempty"`
	Token         string `json:"token,omitempty"`
	ApplicationID string `json:"application_id,omitempty"`
	URL           string `json:"url,omitempty"`
}

// Thread represents a Discord thread.
type Thread struct {
	ID             string          `json:"id"`
	GuildID        string          `json:"guild_id,omitempty"`
	ParentID       string          `json:"parent_id,omitempty"`
	OwnerID        string          `json:"owner_id,omitempty"`
	Type           int             `json:"type"`
	Name           string          `json:"name"`
	LastMessageID  string          `json:"last_message_id,omitempty"`
	MessageCount   int             `json:"message_count,omitempty"`
	MemberCount    int             `json:"member_count,omitempty"`
	ThreadMetadata *ThreadMetadata `json:"thread_metadata,omitempty"`
}

// ThreadMetadata represents metadata about a thread.
type ThreadMetadata struct {
	Archived            bool       `json:"archived"`
	AutoArchiveDuration int        `json:"auto_archive_duration"`
	ArchiveTimestamp    time.Time  `json:"archive_timestamp"`
	Locked              bool       `json:"locked"`
	Invitable           bool       `json:"invitable,omitempty"`
	CreateTimestamp     *time.Time `json:"create_timestamp,omitempty"`
}
