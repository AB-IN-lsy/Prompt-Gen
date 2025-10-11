/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-12 13:08:00
 * @FilePath: \\electron-go-app\\backend\\tests\\unit\\changelog_service_test.go
 * @LastEditTime: 2025-10-12 13:08:00
 */
package unit

import (
	"context"
	"fmt"
	"testing"

	changelog "electron-go-app/backend/internal/domain/changelog"
	"electron-go-app/backend/internal/repository"
	changelogsrv "electron-go-app/backend/internal/service/changelog"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newTestChangelogService(t *testing.T) (*changelogsrv.Service, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	if err := db.AutoMigrate(&changelog.Entry{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := repository.NewChangelogRepository(db)
	svc := changelogsrv.NewService(repo)
	return svc, db
}

func TestChangelogCreateAndListFallback(t *testing.T) {
	svc, db := newTestChangelogService(t)
	t.Cleanup(func() {
		if sql, err := db.DB(); err == nil {
			sql.Close()
		}
	})

	ctx := context.Background()

	entry, err := svc.CreateEntry(ctx, changelogsrv.CreateEntryParams{
		Locale:      "en",
		Badge:       "Docs",
		Title:       "Initial entry",
		Summary:     "English only entry",
		Items:       []string{"Line A", "Line B"},
		PublishedAt: "2025-10-12",
	})
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}
	if entry.Locale != "en" {
		t.Fatalf("expected locale en, got %s", entry.Locale)
	}

	// Request zh-CN entries; fallback should return the English record above.
	list, err := svc.ListEntries(ctx, "zh-CN")
	if err != nil {
		t.Fatalf("list entries: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(list))
	}
	if list[0].Locale != "en" {
		t.Fatalf("expected fallback to en, got %s", list[0].Locale)
	}
	if got := len(list[0].Items); got != 2 {
		t.Fatalf("expected 2 items, got %d", got)
	}
}

func TestChangelogCreateRequiresItems(t *testing.T) {
	svc, db := newTestChangelogService(t)
	t.Cleanup(func() {
		if sql, err := db.DB(); err == nil {
			sql.Close()
		}
	})

	ctx := context.Background()
	if _, err := svc.CreateEntry(ctx, changelogsrv.CreateEntryParams{
		Locale:      "en",
		Badge:       "Docs",
		Title:       "Invalid entry",
		Summary:     "Should fail",
		Items:       []string{},
		PublishedAt: "2025-10-12",
	}); err == nil {
		t.Fatalf("expected error when items missing")
	}
}
