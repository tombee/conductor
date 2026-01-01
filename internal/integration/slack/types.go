package slack

// SlackResponse represents the common Slack API response structure.
// All Slack API responses include an "ok" field indicating success/failure.
type SlackResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// Message represents a Slack message.
type Message struct {
	Type      string `json:"type,omitempty"`
	Channel   string `json:"channel,omitempty"`
	User      string `json:"user,omitempty"`
	Text      string `json:"text,omitempty"`
	Timestamp string `json:"ts,omitempty"`
	ThreadTS  string `json:"thread_ts,omitempty"`
}

// PostMessageResponse represents the response from posting a message.
type PostMessageResponse struct {
	SlackResponse
	Channel   string  `json:"channel"`
	Timestamp string  `json:"ts"`
	Message   Message `json:"message"`
}

// UpdateMessageResponse represents the response from updating a message.
type UpdateMessageResponse struct {
	SlackResponse
	Channel   string  `json:"channel"`
	Timestamp string  `json:"ts"`
	Text      string  `json:"text"`
	Message   Message `json:"message"`
}

// DeleteMessageResponse represents the response from deleting a message.
type DeleteMessageResponse struct {
	SlackResponse
	Channel   string `json:"channel"`
	Timestamp string `json:"ts"`
}

// ReactionResponse represents the response from adding a reaction.
type ReactionResponse struct {
	SlackResponse
}

// FileUploadResponse represents the response from uploading a file.
type FileUploadResponse struct {
	SlackResponse
	File File `json:"file"`
}

// File represents a Slack file.
type File struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Title      string `json:"title"`
	Mimetype   string `json:"mimetype"`
	Filetype   string `json:"filetype"`
	Size       int    `json:"size"`
	URLPrivate string `json:"url_private"`
	Permalink  string `json:"permalink"`
}

// Channel represents a Slack channel/conversation.
type Channel struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IsChannel  bool   `json:"is_channel"`
	IsGroup    bool   `json:"is_group"`
	IsIM       bool   `json:"is_im"`
	IsMpIM     bool   `json:"is_mpim"`
	IsPrivate  bool   `json:"is_private"`
	IsArchived bool   `json:"is_archived"`
	Creator    string `json:"creator,omitempty"`
	NumMembers int    `json:"num_members,omitempty"`
}

// ListChannelsResponse represents the response from listing channels.
type ListChannelsResponse struct {
	SlackResponse
	Channels         []Channel        `json:"channels"`
	ResponseMetadata ResponseMetadata `json:"response_metadata,omitempty"`
}

// CreateChannelResponse represents the response from creating a channel.
type CreateChannelResponse struct {
	SlackResponse
	Channel Channel `json:"channel"`
}

// InviteToChannelResponse represents the response from inviting users to a channel.
type InviteToChannelResponse struct {
	SlackResponse
	Channel Channel `json:"channel"`
}

// User represents a Slack user.
type User struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	RealName string      `json:"real_name,omitempty"`
	Profile  UserProfile `json:"profile,omitempty"`
	IsBot    bool        `json:"is_bot"`
	Deleted  bool        `json:"deleted"`
}

// UserProfile represents a Slack user's profile.
type UserProfile struct {
	DisplayName string `json:"display_name,omitempty"`
	RealName    string `json:"real_name,omitempty"`
	Email       string `json:"email,omitempty"`
	Image48     string `json:"image_48,omitempty"`
}

// ListUsersResponse represents the response from listing users.
type ListUsersResponse struct {
	SlackResponse
	Members          []User           `json:"members"`
	ResponseMetadata ResponseMetadata `json:"response_metadata,omitempty"`
}

// GetUserResponse represents the response from getting a user.
type GetUserResponse struct {
	SlackResponse
	User User `json:"user"`
}

// ResponseMetadata contains pagination information.
type ResponseMetadata struct {
	NextCursor string `json:"next_cursor,omitempty"`
}

// SearchMessagesResponse represents the response from searching messages.
type SearchMessagesResponse struct {
	SlackResponse
	Query    string        `json:"query"`
	Messages SearchResults `json:"messages"`
}

// SearchResults contains the search results.
type SearchResults struct {
	Total      int              `json:"total"`
	Pagination SearchPagination `json:"pagination,omitempty"`
	Matches    []SearchMatch    `json:"matches"`
}

// SearchPagination contains pagination info for search results.
type SearchPagination struct {
	TotalCount int `json:"total_count"`
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	PageCount  int `json:"page_count"`
	First      int `json:"first"`
	Last       int `json:"last"`
}

// SearchMatch represents a single search result match.
type SearchMatch struct {
	Type      string      `json:"type"`
	Channel   ChannelInfo `json:"channel"`
	User      string      `json:"user"`
	Username  string      `json:"username"`
	Text      string      `json:"text"`
	Timestamp string      `json:"ts"`
	Permalink string      `json:"permalink"`
}

// ChannelInfo contains channel information in search results.
type ChannelInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ConversationsOpenResponse represents the response from opening a conversation.
type ConversationsOpenResponse struct {
	SlackResponse
	Channel ConversationChannel `json:"channel"`
}

// ConversationChannel contains basic channel info for opened conversations.
type ConversationChannel struct {
	ID string `json:"id"`
}

// ConversationsHistoryResponse represents the response from getting conversation history.
type ConversationsHistoryResponse struct {
	SlackResponse
	Messages         []Message        `json:"messages"`
	HasMore          bool             `json:"has_more"`
	ResponseMetadata ResponseMetadata `json:"response_metadata,omitempty"`
}
