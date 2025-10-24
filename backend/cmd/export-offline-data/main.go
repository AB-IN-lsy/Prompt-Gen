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
	"electron-go-app/backend/internal/infra/logger"
)

var outputDir = flag.String("output-dir", "", "指定导出 JSON 存放目录，默认使用 LOCAL_BOOTSTRAP_DATA_DIR 或 backend/data/bootstrap")

func main() {
	flag.Parse()

	dest := strings.TrimSpace(*outputDir)
	if dest == "" {
		dest = bootstrapdata.ResolveDataDir()
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
		sugar.Fatalw("init resources failed", "error", err)
	}
	defer func() {
		if cerr := resources.Close(); cerr != nil {
			sugar.Warnw("close resources failed", "error", cerr)
		}
	}()

	if err := bootstrapdata.ExportSnapshot(ctx, resources.DBConn(), bootstrapdata.ExportOptions{
		OutputDir: dest,
		Logger:    sugar.With("component", "export-offline-data"),
	}); err != nil {
		sugar.Fatalw("export snapshot failed", "error", err)
	}

	sugar.Infow("export snapshot completed", "output_dir", dest)
}
