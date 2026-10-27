package ai_test

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	ai "github.com/aosanya/mwanachama-backend-ai"
	"github.com/aosanya/mwanachama-backend-shared/spec"
)

var master = []byte("0123456789abcdef0123456789abcdef")

const syntheticKey = "sk-SYNTHETIC-9f8e7d6c5b4a"

func openDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	return db
}

func testSpec(t *testing.T) *spec.Spec {
	t.Helper()
	s, err := ai.LoadSpec("ai.wakala.json")
	if err != nil {
		t.Fatalf("LoadSpec: %v", err)
	}
	s.Instance = "test"
	return s
}

func newTestManager(t *testing.T, opts ...ai.Option) (ai.Manager, *gorm.DB, *spec.Spec) {
	t.Helper()
	db := openDB(t)
	s := testSpec(t)
	if err := ai.Provision(db, s); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	m, err := ai.NewManager(db, s, opts...)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return m, db, s
}

func validAccount(name string) ai.Account {
	return ai.Account{
		Name: name, Provider: "openai_compatible", BaseURL: "https://models.example.test/v1",
		ModelName: "synthetic-model", Status: "enabled",
	}
}

func ip(n int) *int { return &n }

func iv(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
