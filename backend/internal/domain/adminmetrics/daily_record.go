package adminmetrics

import "time"

// DailyRecord 映射 admin_metrics_daily 表的一行数据。
type DailyRecord struct {
	ID                   uint      `gorm:"column:id;primaryKey"`
	MetricsDate          time.Time `gorm:"column:metrics_date;type:date;uniqueIndex"`
	ActiveUsers          int       `gorm:"column:active_users"`
	GenerateTotal        int       `gorm:"column:generate_total"`
	GenerateSuccess      int       `gorm:"column:generate_success"`
	GenerateSuccessRate  float64   `gorm:"column:generate_success_rate"`
	AverageLatencyMillis float64   `gorm:"column:average_latency_ms"`
	SaveTotal            int       `gorm:"column:save_total"`
	CreatedAt            time.Time `gorm:"column:created_at"`
	UpdatedAt            time.Time `gorm:"column:updated_at"`
}

// TableName 返回对应的 MySQL 表名，避免 Gorm 使用默认命名。
func (DailyRecord) TableName() string {
	return "admin_metrics_daily"
}
