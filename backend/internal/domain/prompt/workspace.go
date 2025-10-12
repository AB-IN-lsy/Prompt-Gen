package prompt

import "time"

// WorkspaceKeyword 描述缓存在 Redis 中的单个关键词。
type WorkspaceKeyword struct {
	Word        string  `json:"word"`
	Source      string  `json:"source,omitempty"`
	Polarity    string  `json:"polarity"`
	Weight      int     `json:"weight,omitempty"`
	DisplayWord string  `json:"display_word,omitempty"`
	Score       float64 `json:"score,omitempty"`
}

// WorkspaceSnapshot 表示 Redis 工作区的整体快照。
type WorkspaceSnapshot struct {
	Token      string             `json:"workspace_token"`
	UserID     uint               `json:"user_id"`
	Topic      string             `json:"topic"`
	Language   string             `json:"language,omitempty"`
	ModelKey   string             `json:"model_key,omitempty"`
	DraftBody  string             `json:"draft_body,omitempty"`
	Positive   []WorkspaceKeyword `json:"positive"`
	Negative   []WorkspaceKeyword `json:"negative"`
	PromptID   uint               `json:"prompt_id,omitempty"`
	Status     string             `json:"status,omitempty"`
	Version    int64              `json:"version"`
	UpdatedAt  time.Time          `json:"updated_at"`
	Attributes map[string]string  `json:"attributes,omitempty"`
}

// PersistenceTask 描述需要异步落库的任务。
type PersistenceTask struct {
	TaskID         string    `json:"task_id"`
	UserID         uint      `json:"user_id"`
	PromptID       uint      `json:"prompt_id,omitempty"`
	WorkspaceToken string    `json:"workspace_token"`
	Publish        bool      `json:"publish"`
	Topic          string    `json:"topic"`
	Body           string    `json:"body"`
	Model          string    `json:"model"`
	Status         string    `json:"status"`
	Tags           []string  `json:"tags,omitempty"`
	RequestedAt    time.Time `json:"requested_at"`
	Action         string    `json:"action,omitempty"`
}

const (
	TaskActionCreate = "create"
	TaskActionUpdate = "update"
)
