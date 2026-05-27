package model

type Channel struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Mode        string `json:"mode"`
	CreatedAt   string `json:"created_at"`
}

type Author struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Message struct {
	ID         int            `json:"id"`
	ChannelID  int            `json:"channel_id"`
	ParentID   int            `json:"parent_id"`
	Title      string         `json:"title"`
	Author     Author         `json:"author"`
	Content    string         `json:"content"`
	Image      string         `json:"image"`
	CreatedAt  string         `json:"created_at"`
	EditedAt   string         `json:"edited_at"`
	ReplyCount int            `json:"reply_count"`
	Reactions  map[string]int `json:"reactions,omitempty"`

	Deleted int8   `json:"-"`
	Country string `json:"-"`
	IP      string `json:"-"`
}
