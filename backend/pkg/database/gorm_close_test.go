package database

import (
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGormRepositoryCloseReleasesSQLPool(t *testing.T) {
	client, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "close.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := client.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pool.Close() })
	if err := pool.Ping(); err != nil {
		t.Fatal(err)
	}
	repo := &GormRepository{GormClient: client}
	if err := repo.Close(); err != nil {
		t.Fatal(err)
	}
	if err := pool.Ping(); err == nil {
		t.Fatal("repository Close left the SQL pool open")
	}
	if err := repo.Close(); err != nil {
		t.Fatalf("repeated close: %v", err)
	}
}
