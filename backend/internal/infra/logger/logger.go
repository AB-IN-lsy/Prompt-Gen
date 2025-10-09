/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 17:53:57
 * @FilePath: \electron-go-app\backend\internal\infra\logger\logger.go
 * @LastEditTime: 2025-10-09 17:54:02
 */
package logger

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	// globalLogger 缓存全局 zap.Logger，避免在业务代码里重复创建实例。
	globalLogger *zap.Logger
	// once 确保初始化逻辑只执行一次。
	once sync.Once
)

// Options 描述日志初始化时可配置的参数。
type Options struct {
	Level      string
	Encoding   string
	FilePath   string
	MaxSize    int
	MaxBackups int
	MaxAge     int
	Compress   bool
}

// Init 初始化全局日志记录器，保证在多次调用时只执行一次构建逻辑。
// 如果初始化失败会返回错误，供上层决定是否降级或退出。
func Init() (*zap.Logger, error) {
	var initErr error
	once.Do(func() {
		opts := loadOptionsFromEnv()
		logger, err := buildLogger(opts)
		if err != nil {
			initErr = err
			return
		}
		globalLogger = logger
	})

	if initErr != nil {
		return nil, initErr
	}

	if globalLogger == nil {
		return nil, errors.New("logger not initialized")
	}

	return globalLogger, nil
}

// L 返回全局 zap.Logger，如果尚未初始化则尝试自动初始化。
// 适合需要使用结构化字段的 场景。
func L() *zap.Logger {
	if globalLogger != nil {
		return globalLogger
	}

	logger, err := Init()
	if err != nil {
		panic(fmt.Sprintf("logger init failed: %v", err))
	}
	return logger
}

// S 返回 SugaredLogger，便于输出结构化键值日志。
// 常用于 handler/service 中编写 `Infof/Warnw` 等语句。
func S() *zap.SugaredLogger {
	return L().Sugar()
}

// Sync 刷新缓冲区，通常在进程退出前调用。
func Sync() {
	if globalLogger != nil {
		_ = globalLogger.Sync()
	}
}

// loadOptionsFromEnv 从环境变量解析日志配置，提供 JSON/console 编码、文件滚动等选项。
// 如果缺失则回退到合理默认值。
func loadOptionsFromEnv() Options {
	opts := Options{
		Level:      strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL"))),
		Encoding:   strings.ToLower(strings.TrimSpace(os.Getenv("LOG_ENCODING"))),
		FilePath:   strings.TrimSpace(os.Getenv("LOG_FILE")),
		MaxSize:    20,
		MaxBackups: 5,
		MaxAge:     15,
		Compress:   true,
	}

	if opts.Level == "" {
		opts.Level = "info"
	}
	if opts.Encoding == "" {
		opts.Encoding = "json"
	}
	if opts.FilePath == "" {
		opts.FilePath = filepath.Join("logs", "app.log")
	}

	if val := strings.TrimSpace(os.Getenv("LOG_MAX_SIZE")); val != "" {
		if parsed, err := parsePositiveInt(val); err == nil {
			opts.MaxSize = parsed
		}
	}
	if val := strings.TrimSpace(os.Getenv("LOG_MAX_BACKUPS")); val != "" {
		if parsed, err := parsePositiveInt(val); err == nil {
			opts.MaxBackups = parsed
		}
	}
	if val := strings.TrimSpace(os.Getenv("LOG_MAX_AGE")); val != "" {
		if parsed, err := parsePositiveInt(val); err == nil {
			opts.MaxAge = parsed
		}
	}
	if val := strings.TrimSpace(os.Getenv("LOG_COMPRESS")); val != "" {
		opts.Compress = val == "1" || strings.EqualFold(val, "true")
	}

	return opts
}

// buildLogger 根据 Options 构建 zap.Logger，并拼接多个输出 Core。
// 默认包含彩色控制台输出，若设置 LOG_FILE 则启用带滚动策略的文件输出。
func buildLogger(opts Options) (*zap.Logger, error) {
	lvl := zapcore.InfoLevel
	if err := lvl.Set(opts.Level); err != nil {
		return nil, fmt.Errorf("invalid log level %q: %w", opts.Level, err)
	}

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.EncodeTime = zapcore.TimeEncoderOfLayout(time.RFC3339Nano)
	encoderCfg.TimeKey = "ts"
	encoderCfg.EncodeDuration = zapcore.StringDurationEncoder

	cores := []zapcore.Core{}

	// 文件输出，带滚动策略。
	if opts.FilePath != "" {
		if err := ensureDir(filepath.Dir(opts.FilePath)); err != nil {
			return nil, fmt.Errorf("logger create dir: %w", err)
		}
		lumber := &lumberjack.Logger{
			Filename:   opts.FilePath,
			MaxSize:    opts.MaxSize,
			MaxBackups: opts.MaxBackups,
			MaxAge:     opts.MaxAge,
			Compress:   opts.Compress,
		}
		fileWriter := zapcore.AddSync(lumber)

		var fileEncoder zapcore.Encoder
		if opts.Encoding == "console" {
			fileEncoder = zapcore.NewConsoleEncoder(encoderCfg)
		} else {
			fileEncoder = zapcore.NewJSONEncoder(encoderCfg)
		}
		cores = append(cores, zapcore.NewCore(fileEncoder, fileWriter, lvl))
	}

	// 控制台输出，默认开启，便于本地调试。
	consoleEncoderCfg := encoderCfg
	consoleEncoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	consoleCore := zapcore.NewCore(
		zapcore.NewConsoleEncoder(consoleEncoderCfg),
		zapcore.AddSync(os.Stdout),
		lvl,
	)
	cores = append(cores, consoleCore)

	combined := zapcore.NewTee(cores...)
	logger := zap.New(combined, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	return logger, nil
}

// ensureDir 若目录不存在则创建，用于保证日志文件目录存在。
func ensureDir(dir string) error {
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

// parsePositiveInt 尝试解析大于 0 的整数，供滚动配置使用。
func parsePositiveInt(val string) (int, error) {
	if val == "" {
		return 0, fmt.Errorf("empty value")
	}
	var parsed int
	if _, err := fmt.Sscanf(val, "%d", &parsed); err != nil {
		return 0, err
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("value must be positive")
	}
	return parsed, nil
}
