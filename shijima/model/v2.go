package model

const PageSize = 30

type Thread struct {
	No      uint   `json:"no"`
	Title   string `json:"t,omitempty"`
	Name    string `json:"n,omitempty"`
	Created string `json:"ts"`
	ID      string `json:"id"`
	Image   string `json:"p,omitempty"`
	Content string `json:"txt"`
	ReplyTo uint   `json:"r,omitempty"`
	Deleted int8   `json:"-"`
	Country string `json:"-"`
	IP      string `json:"-"`
}

type BoardThread struct {
	Thread
	ReplyCount int       `json:"num"`
	Replies    []*Thread `json:"list,omitempty"`
}

