package adminmetrics

import (
	"context"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	adminmetricsdomain "electron-go-app/backend/internal/domain/adminmetrics"
	appLogger "electron-go-app/backend/internal/infra/logger"

	"go.uber.org/zap"
)

const (
	defaultRefreshInterval = 5 * time.Minute
	defaultRetentionDays   = 7
)

// ActivityKind 表示打点事件的类型，区分生成与保存行为。
type ActivityKind string

const (
	ActivityGenerate ActivityKind = "generate"
	ActivitySave     ActivityKind = "save"
)

// ActivityEvent 描述后台统计接收到的一次业务事件。
type ActivityEvent struct {
	UserID    uint
	Kind      ActivityKind
	Success   bool
	Duration  time.Duration
	Timestamp time.Time
}

// Config 用于配置后台统计的缓存刷新频率与保留窗口。
type Config struct {
	RefreshInterval time.Duration
	RetentionDays   int
}

// Snapshot 汇总最近若干天的指标快照，供接口直接返回。
type Snapshot struct {
	RefreshedAt time.Time     `json:"refreshed_at"`
	RangeDays   int           `json:"range_days"`
	Daily       []DailyMetric `json:"daily"`
	Totals      Totals        `json:"totals"`
}

// DailyMetric 描述单日的关键指标。
type DailyMetric struct {
	Date                 string  `json:"date"`
	ActiveUsers          int     `json:"active_users"`
	GenerateRequests     int     `json:"generate_requests"`
	GenerateSuccess      int     `json:"generate_success"`
	GenerateSuccessRate  float64 `json:"generate_success_rate"`
	AverageLatencyMillis float64 `json:"average_latency_ms"`
	SaveRequests         int     `json:"save_requests"`
}

// Totals 汇总区间内的整体表现。
type Totals struct {
	ActiveUsers          int     `json:"active_users"`
	GenerateRequests     int     `json:"generate_requests"`
	GenerateSuccess      int     `json:"generate_success"`
	GenerateSuccessRate  float64 `json:"generate_success_rate"`
	AverageLatencyMillis float64 `json:"average_latency_ms"`
	SaveRequests         int     `json:"save_requests"`
}

// dailyBucket 存储某一天的原始事件聚合，用于后续生成快照。
type dailyBucket struct {
	date            time.Time         // 该桶对应的日期（零点时间）。
	activeUsers     map[uint]struct{} // 当天活跃用户集合，map 用于去重。
	generateTotal   int               // 生成请求总次数。
	generateSuccess int               // 生成成功次数。
	latencySum      time.Duration     // 生成请求耗时累加值。
	latencySamples  int               // 记录了耗时的样本数量。
	saveTotal       int               // 保存请求总次数。
}

// Service 通过内存缓存维护管理员指标，并周期性刷新快照。
type Service struct {
	cfg      Config                  // 模块配置，包含刷新周期与保留天数。
	logger   *zap.SugaredLogger      // 统一日志实例。
	repo     Repository              // 指标持久化仓储。
	started  atomic.Bool             // 标记调度器是否已启动，避免重复启动。
	mu       sync.RWMutex            // 保护 buckets 与 snapshot 的读写锁。
	buckets  map[string]*dailyBucket // 日期 -> 日维度指标桶。
	snapshot Snapshot                // 最近一次生成的快照。
	dirty    bool                    // 数据是否被新事件污染，需要重算快照。
}

// Repository 抽象指标模块访问数据库的必要方法。
type Repository interface {
	LoadEventsRange(ctx context.Context, start, end time.Time) ([]adminmetricsdomain.EventRecord, error)
	AppendEvent(ctx context.Context, event adminmetricsdomain.EventRecord) error
	UpsertDaily(ctx context.Context, record adminmetricsdomain.DailyRecord) error
	DeleteEventsBefore(ctx context.Context, boundary time.Time) error
}

// NewService 创建后台指标服务，若 logger 为空则使用默认日志实例。
func NewService(cfg Config, logger *zap.SugaredLogger, repo Repository) *Service {
	if logger == nil {
		logger = appLogger.S().With("component", "service.adminmetrics")
	}
	if cfg.RefreshInterval <= 0 {
		cfg.RefreshInterval = defaultRefreshInterval
	}
	if cfg.RetentionDays <= 0 {
		cfg.RetentionDays = defaultRetentionDays
	}
	return &Service{
		cfg:     cfg,
		logger:  logger,
		repo:    repo,
		buckets: make(map[string]*dailyBucket),
		dirty:   true,
		snapshot: Snapshot{
			RefreshedAt: time.Time{},
			RangeDays:   cfg.RetentionDays,
			Daily:       []DailyMetric{},
			Totals:      Totals{},
		},
	}
}

// Start 启动定时刷新任务，根据配置周期重建快照。
func (s *Service) Start(ctx context.Context) {
	// 确保只启动一次。
	if !s.started.CompareAndSwap(false, true) {
		return
	}
	s.logger.Infow("admin metrics scheduler started", "interval", s.cfg.RefreshInterval, "retention_days", s.cfg.RetentionDays)
	s.hydrateFromRepository(ctx)
	s.refreshSnapshot(ctx, time.Now())

	ticker := time.NewTicker(s.cfg.RefreshInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				s.logger.Infow("admin metrics scheduler stopped")
				return
			case now := <-ticker.C:
				s.refreshSnapshot(ctx, now)
			}
		}
	}()
}

// hydrateFromRepository 会从持久化层加载历史事件，恢复内存中的指标桶。
func (s *Service) hydrateFromRepository(ctx context.Context) {
	if s.repo == nil {
		return
	}

	now := time.Now()
	windowStart := truncateToDay(now).AddDate(0, 0, -(s.cfg.RetentionDays - 1))
	records, err := s.repo.LoadEventsRange(ctx, windowStart, now)
	if err != nil {
		s.logger.Warnw("load admin metrics events failed", "error", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, record := range records {
		kind := ActivityKind(record.Kind)
		if kind != ActivityGenerate && kind != ActivitySave {
			continue
		}
		event := ActivityEvent{
			UserID:    record.UserID,
			Kind:      kind,
			Success:   record.Success,
			Timestamp: record.OccurredAt,
		}
		if record.DurationMillis != nil {
			event.Duration = time.Duration(*record.DurationMillis) * time.Millisecond
		}
		s.applyEventLocked(event)
	}
	if len(records) > 0 {
		s.dirty = true
	}
}

// RecordActivity 接收一条业务事件并写入内存缓存，后续供快照刷新使用。
func (s *Service) RecordActivity(event ActivityEvent) {
	if event.UserID == 0 {
		return
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	s.mu.Lock()
	s.applyEventLocked(event)
	s.dirty = true
	s.mu.Unlock()

	s.persistEvent(event)
}

// applyEventLocked 在已加锁的情况下将事件写入指定日期的桶。
func (s *Service) applyEventLocked(event ActivityEvent) {
	day := truncateToDay(event.Timestamp)
	key := day.Format("2006-01-02")

	bucket, ok := s.buckets[key]
	if !ok {
		bucket = &dailyBucket{
			date:        day,
			activeUsers: make(map[uint]struct{}),
		}
		s.buckets[key] = bucket
	}
	bucket.activeUsers[event.UserID] = struct{}{}

	switch event.Kind {
	case ActivityGenerate:
		bucket.generateTotal++
		if event.Success {
			bucket.generateSuccess++
		}
		if event.Duration > 0 {
			bucket.latencySum += event.Duration
			bucket.latencySamples++
		}
	case ActivitySave:
		bucket.saveTotal++
	}
}

// persistEvent 将最新事件写入数据库，确保服务重启后可以重放。
func (s *Service) persistEvent(event ActivityEvent) {
	if s.repo == nil {
		return
	}

	record := adminmetricsdomain.EventRecord{
		UserID:     event.UserID,
		Kind:       string(event.Kind),
		Success:    event.Success,
		OccurredAt: event.Timestamp,
	}
	if event.Duration > 0 {
		ms := event.Duration.Milliseconds()
		record.DurationMillis = &ms
	}

	if err := s.repo.AppendEvent(context.Background(), record); err != nil {
		s.logger.Warnw("append admin metrics event failed", "error", err)
	}
}

// persistDailyMetrics 将最新的每日汇总写入 MySQL。
func (s *Service) persistDailyMetrics(ctx context.Context, metrics []DailyMetric) {
	if s.repo == nil || len(metrics) == 0 {
		return
	}

	for _, metric := range metrics {
		if metric.Date == "" {
			continue
		}
		date, err := time.Parse("2006-01-02", metric.Date)
		if err != nil {
			s.logger.Warnw("parse metrics date failed", "date", metric.Date, "error", err)
			continue
		}

		record := adminmetricsdomain.DailyRecord{
			MetricsDate:          date,
			ActiveUsers:          metric.ActiveUsers,
			GenerateTotal:        metric.GenerateRequests,
			GenerateSuccess:      metric.GenerateSuccess,
			GenerateSuccessRate:  metric.GenerateSuccessRate,
			AverageLatencyMillis: metric.AverageLatencyMillis,
			SaveTotal:            metric.SaveRequests,
		}
		if err := s.repo.UpsertDaily(ctx, record); err != nil {
			s.logger.Warnw("upsert admin metrics daily failed", "date", metric.Date, "error", err)
		}
	}
}

// cleanExpiredEvents 清理超出保留窗口的事件记录。
func (s *Service) cleanExpiredEvents(ctx context.Context, reference time.Time) {
	if s.repo == nil {
		return
	}
	boundary := truncateToDay(reference).AddDate(0, 0, -s.cfg.RetentionDays)
	if err := s.repo.DeleteEventsBefore(ctx, boundary); err != nil {
		s.logger.Warnw("delete old admin metrics events failed", "boundary", boundary, "error", err)
	}
}

// Snapshot 返回最新快照，如有必要会触发增量刷新。
func (s *Service) Snapshot() Snapshot {
	now := time.Now()

	var updatedDaily []DailyMetric
	s.mu.Lock()
	// 判断是否需要重新整理
	if s.dirty || now.Sub(s.snapshot.RefreshedAt) >= s.cfg.RefreshInterval {
		updatedDaily = s.rebuildSnapshotLocked(now)
	}
	result := cloneSnapshot(s.snapshot)
	s.mu.Unlock()

	if len(updatedDaily) > 0 {
		s.persistDailyMetrics(context.Background(), updatedDaily)
		s.cleanExpiredEvents(context.Background(), now)
	}

	return result
}

// refreshSnapshot 对外暴露的刷新入口，获取写锁后重算最新快照
// 并持久化增量数据。
func (s *Service) refreshSnapshot(ctx context.Context, now time.Time) {
	s.mu.Lock()
	updatedDaily := s.rebuildSnapshotLocked(now)
	s.mu.Unlock()

	s.persistDailyMetrics(ctx, updatedDaily)
	s.cleanExpiredEvents(ctx, now)
}

// rebuildSnapshotLocked 在已持有写锁的前提下，根据当前桶数据生成快照。
func (s *Service) rebuildSnapshotLocked(now time.Time) []DailyMetric {
	windowStart := truncateToDay(now).AddDate(0, 0, -(s.cfg.RetentionDays - 1))
	for key, bucket := range s.buckets {
		if bucket.date.Before(windowStart) {
			delete(s.buckets, key)
		}
	}

	keys := make([]string, 0, len(s.buckets))
	for key := range s.buckets {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	daily := make([]DailyMetric, 0, len(keys))
	var totalUsers = make(map[uint]struct{})
	var totalGenerate, totalSuccess, totalSave int
	var totalLatency time.Duration
	var totalSamples int

	for _, key := range keys {
		bucket := s.buckets[key]
		for userID := range bucket.activeUsers {
			totalUsers[userID] = struct{}{}
		}

		successRate := computeRate(bucket.generateSuccess, bucket.generateTotal)
		avgLatency := computeAverageMillis(bucket.latencySum, bucket.latencySamples)

		daily = append(daily, DailyMetric{
			Date:                 key,
			ActiveUsers:          len(bucket.activeUsers),
			GenerateRequests:     bucket.generateTotal,
			GenerateSuccess:      bucket.generateSuccess,
			GenerateSuccessRate:  successRate,
			AverageLatencyMillis: avgLatency,
			SaveRequests:         bucket.saveTotal,
		})

		totalGenerate += bucket.generateTotal
		totalSuccess += bucket.generateSuccess
		totalSave += bucket.saveTotal
		totalLatency += bucket.latencySum
		totalSamples += bucket.latencySamples
	}

	s.snapshot = Snapshot{
		RefreshedAt: now,
		RangeDays:   s.cfg.RetentionDays,
		Daily:       daily,
		Totals: Totals{
			ActiveUsers:          len(totalUsers),
			GenerateRequests:     totalGenerate,
			GenerateSuccess:      totalSuccess,
			GenerateSuccessRate:  computeRate(totalSuccess, totalGenerate),
			AverageLatencyMillis: computeAverageMillis(totalLatency, totalSamples),
			SaveRequests:         totalSave,
		},
	}
	s.dirty = false
	return append([]DailyMetric(nil), daily...)
}

// cloneSnapshot 复制快照，避免调用方篡改内部引用。
func cloneSnapshot(src Snapshot) Snapshot {
	dst := src
	if len(src.Daily) > 0 {
		dst.Daily = append([]DailyMetric(nil), src.Daily...)
	}
	return dst
}

// truncateToDay 将时间截断为所在自然日的零点。
func truncateToDay(t time.Time) time.Time {
	loc := time.UTC
	if t.Location() != nil {
		loc = t.Location()
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}

// computeRate 计算成功率，处理分母为零的场景。
func computeRate(success, total int) float64 {
	if total <= 0 || success < 0 {
		return 0
	}
	return float64(success) / float64(total)
}

// computeAverageMillis 根据耗时总和与样本数计算毫秒平均值。
func computeAverageMillis(sum time.Duration, samples int) float64 {
	if samples <= 0 || sum <= 0 {
		return 0
	}
	return float64(sum.Milliseconds()) / float64(samples)
}
