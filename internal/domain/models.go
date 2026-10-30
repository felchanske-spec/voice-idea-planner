package domain

import (
	"errors"
	"time"
)

var (
	ErrIdeaNotFound   = errors.New("idea not found")
	ErrEmptyVoiceText = errors.New("voice message transcription is empty")
	ErrUserNotFound   = errors.New("user not found")
)

type User struct {
	ID        int64
	State     string
	CreatedAt time.Time
}

type Idea struct {
	ID           int64
	UserID       int64
	Title        string
	RawText      string
	MarkDownText string
	MermaidCode  string
	Sections     IdeaSection
	CreatedAt    time.Time
}

type IdeaSection struct {
	MVP   string
	Stack string
	Risk  string
}
