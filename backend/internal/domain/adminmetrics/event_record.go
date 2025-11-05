package adminmetrics

import "time"

// EventRecord 映射 admin_metrics_events 表的一次原始事件。
type EventRecord struct {
	ID             uint       `gorm:"column:id;primaryKey"`
	UserID         uint       `gorm:"column:user_id"`
	Kind           string     `gorm:"column:kind"`
	Success        bool       `gorm:"column:success"`
	DurationMillis *int64     `gorm:"column:duration_ms"`
	OccurredAt     time.Time  `gorm:"column:occurred_at"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
	UpdatedAt      *time.Time `gorm:"column:updated_at"`
}

// TableName 返回事件表的名称。
func (EventRecord) TableName() string {
	return "admin_metrics_events"
}
