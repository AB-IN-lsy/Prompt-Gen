package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"electron-go-app/backend/internal/app"
	"electron-go-app/backend/internal/bootstrapdata"
	"electron-go-app/backend/internal/config"
	changelog "electron-go-app/backend/internal/domain/changelog"
	promptdomain "electron-go-app/backend/internal/domain/prompt"
	"electron-go-app/backend/internal/infra/logger"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	outputPath = flag.String("output", "", "指定生成的 SQLite 文件路径")
	dataDir    = flag.String("data-dir", "", "指定预置数据目录，默认读取 LOCAL_BOOTSTRAP_DATA_DIR")
)

// main 是离线引导工具入口，用于生成带有预置数据的本地 SQLite 文件。
func main() {
	flag.Parse()

	ensureLocalMode()

	if *outputPath != "" {
		if err := os.Setenv("LOCAL_SQLITE_PATH", strings.TrimSpace(*outputPath)); err != nil {
			panic(fmt.Sprintf("set LOCAL_SQLITE_PATH failed: %v", err))
		}
	}
	if *dataDir != "" {
		if err := os.Setenv("LOCAL_BOOTSTRAP_DATA_DIR", strings.TrimSpace(*dataDir)); err != nil {
			panic(fmt.Sprintf("set LOCAL_BOOTSTRAP_DATA_DIR failed: %v", err))
		}
	}

	zapLogger, err := logger.Init()
	if err != nil {
		panic(fmt.Sprintf("init logger failed: %v", err))
	}
	defer logger.Sync()
	sugar := zapLogger.Sugar()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	resources, err := app.InitResources(ctx)
	if err != nil {
		sugar.Fatalw("initialise resources failed", "error", err)
	}
	defer func() {
		if closeErr := resources.Close(); closeErr != nil {
			sugar.Warnw("close resources failed", "error", closeErr)
		}
	}()

	if err := reportSeedSummary(ctx, resources.DBConn(), sugar); err != nil {
		sugar.Warnw("report seed summary failed", "error", err)
	}

	sugar.Infow(
		"offline database ready",
		"sqlite_path", resources.Config.Local.DBPath,
		"data_dir", bootstrapdata.ResolveDataDir(),
	)
}

// ensureLocalMode 确保命令在本地模式下运行，便于自动应用 SQLite 与预置数据逻辑。
func ensureLocalMode() {
	if mode := strings.TrimSpace(os.Getenv("APP_MODE")); !strings.EqualFold(mode, config.ModeLocal) {
		if err := os.Setenv("APP_MODE", config.ModeLocal); err != nil {
			panic(fmt.Sprintf("set APP_MODE failed: %v", err))
		}
	}
}

// reportSeedSummary 统计关键表的记录数，便于调用者确认导入结果。
func reportSeedSummary(ctx context.Context, db *gorm.DB, sugar *zap.SugaredLogger) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}

	var promptCount int64
	if err := db.WithContext(ctx).Model(&promptdomain.PublicPrompt{}).Count(&promptCount).Error; err != nil {
		return fmt.Errorf("count public prompts: %w", err)
	}

	var changelogCount int64
	if err := db.WithContext(ctx).Model(&changelog.Entry{}).Count(&changelogCount).Error; err != nil {
		return fmt.Errorf("count changelog entries: %w", err)
	}

	sugar.Infow(
		"seed summary",
		"public_prompts", promptCount,
		"changelog_entries", changelogCount,
	)
	return nil
}
