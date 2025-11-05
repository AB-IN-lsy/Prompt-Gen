package repository

import (
	"context"
	"time"

	adminmetrics "electron-go-app/backend/internal/domain/adminmetrics"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AdminMetricsRepository 负责管理员运营指标相关的持久化操作。
type AdminMetricsRepository struct {
	db *gorm.DB
}

// NewAdminMetricsRepository 构造指标仓储，复用主数据库连接。
func NewAdminMetricsRepository(db *gorm.DB) *AdminMetricsRepository {
	if db == nil {
		return nil
	}
	return &AdminMetricsRepository{db: db}
}

// LoadEventsRange 按时间范围读取原始事件，用于服务启动时恢复内存缓存。
func (r *AdminMetricsRepository) LoadEventsRange(ctx context.Context, start, end time.Time) ([]adminmetrics.EventRecord, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	var records []adminmetrics.EventRecord
	query := r.db.WithContext(ctx).Model(&adminmetrics.EventRecord{})
	if !start.IsZero() {
		query = query.Where("occurred_at >= ?", start)
	}
	if !end.IsZero() {
		query = query.Where("occurred_at <= ?", end)
	}
	if err := query.Order("occurred_at ASC").Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

// AppendEvent 将单条原始事件写入事件表，供后续重放。
func (r *AdminMetricsRepository) AppendEvent(ctx context.Context, event adminmetrics.EventRecord) error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.WithContext(ctx).Create(&event).Error
}

// UpsertDaily 根据唯一日期写入或更新每日汇总指标。
func (r *AdminMetricsRepository) UpsertDaily(ctx context.Context, record adminmetrics.DailyRecord) error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "metrics_date"}},
			DoUpdates: clause.AssignmentColumns([]string{"active_users", "generate_total", "generate_success", "generate_success_rate", "average_latency_ms", "save_total"}),
		}).
		Create(&record).Error
}

// DeleteEventsBefore 清理早于指定时间的事件，控制表规模。
func (r *AdminMetricsRepository) DeleteEventsBefore(ctx context.Context, boundary time.Time) error {
	if r == nil || r.db == nil {
		return nil
	}
	if boundary.IsZero() {
		return nil
	}
	return r.db.WithContext(ctx).
		Where("occurred_at < ?", boundary).
		Delete(&adminmetrics.EventRecord{}).Error
}
