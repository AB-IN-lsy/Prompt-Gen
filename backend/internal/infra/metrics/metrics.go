package metrics

import (
	"strings"
	"sync"
	"time"

	modeldomain "electron-go-app/backend/internal/infra/model/deepseek"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

var (
	registerOnce           sync.Once
	promptGenerateRequests *prometheus.CounterVec
	promptGenerateDuration *prometheus.HistogramVec
	promptGenerateTokens   *prometheus.CounterVec
	promptSaveRequests     *prometheus.CounterVec
	defaultDurationBuckets = prometheus.DefBuckets
)

const (
	namespaceMetrics = "promptgen"
)

// MustRegister 初始化 Prometheus 指标并注册 Go 运行时采样器，需在应用启动阶段调用一次。
func MustRegister() {
	registerOnce.Do(func() {
		promptGenerateRequests = registerCounterVec(
			prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: namespaceMetrics,
					Subsystem: "generation",
					Name:      "requests_total",
					Help:      "生成接口的调用次数，按执行状态统计。",
				},
				[]string{"status"},
			),
		)
		promptGenerateDuration = registerHistogramVec(
			prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Namespace: namespaceMetrics,
					Subsystem: "generation",
					Name:      "duration_seconds",
					Help:      "生成接口的模型调用耗时，按模型区分。",
					Buckets:   defaultDurationBuckets,
				},
				[]string{"model"},
			),
		)
		promptGenerateTokens = registerCounterVec(
			prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: namespaceMetrics,
					Subsystem: "generation",
					Name:      "tokens_total",
					Help:      "生成接口消耗的 token 数量，按 token 类型拆分。",
				},
				[]string{"token_type"},
			),
		)
		promptSaveRequests = registerCounterVec(
			prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: namespaceMetrics,
					Subsystem: "persistence",
					Name:      "save_requests_total",
					Help:      "保存或发布 Prompt 的调用次数，按结果分类。",
				},
				[]string{"result"},
			),
		)

		registerRuntimeCollectors()
	})
}

// ObservePromptGenerate 记录生成接口的调用结果、耗时与 token 消耗。
func ObservePromptGenerate(status, model string, duration time.Duration, usage *modeldomain.ChatCompletionUsage) {
	if promptGenerateRequests == nil || promptGenerateDuration == nil {
		return
	}
	label := normalizeLabel(status, "unknown")
	promptGenerateRequests.WithLabelValues(label).Inc()

	modelLabel := normalizeLabel(model, "unspecified")
	promptGenerateDuration.WithLabelValues(modelLabel).Observe(duration.Seconds())

	if promptGenerateTokens == nil || usage == nil {
		return
	}
	if usage.PromptTokens > 0 {
		promptGenerateTokens.WithLabelValues("prompt").Add(float64(usage.PromptTokens))
	}
	if usage.CompletionTokens > 0 {
		promptGenerateTokens.WithLabelValues("completion").Add(float64(usage.CompletionTokens))
	}
	if usage.TotalTokens > 0 {
		promptGenerateTokens.WithLabelValues("total").Add(float64(usage.TotalTokens))
	}
}

// RecordPromptSave 记录保存或发布 Prompt 的结果分布。
func RecordPromptSave(result string) {
	if promptSaveRequests == nil {
		return
	}
	promptSaveRequests.WithLabelValues(normalizeLabel(result, "unknown")).Inc()
}

func normalizeLabel(value string, fallback string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return fallback
}

func registerCounterVec(vec *prometheus.CounterVec) *prometheus.CounterVec {
	if err := prometheus.Register(vec); err != nil {
		if existing := alreadyRegisteredCounterVec(err); existing != nil {
			return existing
		}
		panic(err)
	}
	return vec
}

func registerHistogramVec(vec *prometheus.HistogramVec) *prometheus.HistogramVec {
	if err := prometheus.Register(vec); err != nil {
		if existing := alreadyRegisteredHistogramVec(err); existing != nil {
			return existing
		}
		panic(err)
	}
	return vec
}

func registerRuntimeCollectors() {
	if err := prometheus.Register(collectors.NewGoCollector()); err != nil {
		if !isAlreadyRegistered(err) {
			panic(err)
		}
	}
	if err := prometheus.Register(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})); err != nil {
		if !isAlreadyRegistered(err) {
			panic(err)
		}
	}
}

func alreadyRegisteredCounterVec(err error) *prometheus.CounterVec {
	if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
		if existing, ok := are.ExistingCollector.(*prometheus.CounterVec); ok {
			return existing
		}
	}
	return nil
}

func alreadyRegisteredHistogramVec(err error) *prometheus.HistogramVec {
	if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
		if existing, ok := are.ExistingCollector.(*prometheus.HistogramVec); ok {
			return existing
		}
	}
	return nil
}

func isAlreadyRegistered(err error) bool {
	_, ok := err.(prometheus.AlreadyRegisteredError)
	return ok
}
