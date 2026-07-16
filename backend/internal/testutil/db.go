//go:build integration

package testutil

import (
	"testing"

	"github.com/insmtx/Leros/backend/config"
	infradb "github.com/insmtx/Leros/backend/internal/infra/db"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Setup(t *testing.T) *gorm.DB {
	t.Helper()

	cfg := LoadTestConfig(t)
	db, err := infradb.InitDB(*cfg.Database, cfg.LLM)
	if err != nil {
		t.Fatalf("init test db: %v", err)
	}

	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })

	return tx
}

func SetupTestDB(t *testing.T, cfg *config.DatabaseConfig) *gorm.DB {
	t.Helper()

	database, err := gorm.Open(postgres.Open(cfg.URL), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}

	tx := database.Begin()
	t.Cleanup(func() { tx.Rollback() })

	return tx
}

func SetupTestDBWithMigrations(t *testing.T, cfg *config.DatabaseConfig, models ...interface{}) *gorm.DB {
	t.Helper()

	db := SetupTestDB(t, cfg)
	if len(models) > 0 {
		if err := db.AutoMigrate(models...); err != nil {
			t.Fatalf("migrate test db: %v", err)
		}
	}

	return db
}
