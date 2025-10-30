package prompt

import "time"

// WorkspaceKeyword 描述缓存在 Redis 中的单个关键词。
type WorkspaceKeyword struct {
	Word        string  `json:"word"`                   // 关键词内容
	Source      string  `json:"source,omitempty"`       // 来源：manual/model/local
	Polarity    string  `json:"polarity"`               // 正向或负向
	Weight      int     `json:"weight,omitempty"`       // 权重（0-5）
	DisplayWord string  `json:"display_word,omitempty"` // 展示用文本（可与 Word 不同）
	Score       float64 `json:"score,omitempty"`        // 推荐分数
}

// WorkspaceSnapshot 表示 Redis 工作区的整体快照。
type WorkspaceSnapshot struct {
	Token      string             `json:"workspace_token"`      // Redis 工作区 token
	UserID     uint               `json:"user_id"`              // 所属用户
	Topic      string             `json:"topic"`                // 当前主题
	Language   string             `json:"language,omitempty"`   // 语言（可选）
	ModelKey   string             `json:"model_key,omitempty"`  // 选中的模型
	DraftBody  string             `json:"draft_body,omitempty"` // 草稿正文
	Positive   []WorkspaceKeyword `json:"positive"`             // 正向关键词列表
	Negative   []WorkspaceKeyword `json:"negative"`             // 负向关键词列表
	PromptID   uint               `json:"prompt_id,omitempty"`  // 关联 Prompt ID
	Status     string             `json:"status,omitempty"`     // 草稿状态
	Version    int64              `json:"version"`              // 快照版本号
	UpdatedAt  time.Time          `json:"updated_at"`           // 最近更新时间
	Attributes map[string]string  `json:"attributes,omitempty"` // 额外属性（标签、补充说明等）
}

// PersistenceTask 描述需要异步落库的任务。
type PersistenceTask struct {
	TaskID         string    `json:"task_id"`             // 队列任务 ID
	UserID         uint      `json:"user_id"`             // 用户 ID
	PromptID       uint      `json:"prompt_id,omitempty"` // 目标 Prompt（为空表示创建）
	WorkspaceToken string    `json:"workspace_token"`     // 工作区 token
	Publish        bool      `json:"publish"`             // 是否发布
	Topic          string    `json:"topic"`               // 主题
	Body           string    `json:"body"`                // 正文
	Instructions   string    `json:"instructions"`        // 补充要求
	Model          string    `json:"model"`               // 模型键
	Status         string    `json:"status"`              // 目标状态
	Tags           []string  `json:"tags,omitempty"`      // 标签
	RequestedAt    time.Time `json:"requested_at"`        // 入队时间
	Action         string    `json:"action,omitempty"`    // 动作类型（create/update）
}

const (
	TaskActionCreate = "create"
	TaskActionUpdate = "update"
)
