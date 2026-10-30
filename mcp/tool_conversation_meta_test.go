package mcp_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	gosdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"

	ai "github.com/aosanya/mwanachama-backend-ai"
	aimcp "github.com/aosanya/mwanachama-backend-ai/mcp"
)

type conversationRecorder struct {
	ai.Manager
	mu   sync.Mutex
	seen []string
}

func (r *conversationRecorder) ListAccounts(ctx context.Context, agencyID string) ([]ai.Account, error) {
	r.mu.Lock()
	r.seen = append(r.seen, dispatch.ConversationOf(ctx))
	r.mu.Unlock()
	return r.Manager.ListAccounts(ctx, agencyID)
}

func newManager(t *testing.T) ai.Manager {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	pool.SetMaxOpenConns(1)
	s, err := ai.LoadSpec("../ai.wakala.json")
	if err != nil {
		t.Fatal(err)
	}
	s.Instance = "metatest"
	if err := ai.Provision(db, s); err != nil {
		t.Fatal(err)
	}
	m, err := ai.NewManager(db, s)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func conversationsSeenBy(t *testing.T, meta gosdkmcp.Meta) []string {
	t.Helper()
	recorder := &conversationRecorder{Manager: newManager(t)}
	server := gosdkmcp.NewServer(&gosdkmcp.Implementation{Name: "test", Version: "0.0.0"}, nil)
	aimcp.RegisterTools(server, recorder)
	ct, st := gosdkmcp.NewInMemoryTransports()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := gosdkmcp.NewClient(&gosdkmcp.Implementation{Name: "c", Version: "0.0.0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	if _, err := cs.CallTool(ctx, &gosdkmcp.CallToolParams{
		Meta:      meta,
		Name:      "ai_list_accounts",
		Arguments: map[string]any{"agencyID": "ag-1"},
	}); err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	return recorder.seen
}

func TestAToolCallCarryingMetaConversationIdReachesTheManagerOnItsContext(t *testing.T) {
	seen := conversationsSeenBy(t, gosdkmcp.Meta{dispatch.ConversationMetaKey: "chat-42"})
	if len(seen) != 1 || seen[0] != "chat-42" {
		t.Fatalf("the manager saw conversations %q, want exactly [chat-42]", seen)
	}
}

func TestAToolCallCarryingNoMetaReachesTheManagerWithNoConversation(t *testing.T) {
	seen := conversationsSeenBy(t, nil)
	if len(seen) != 1 || seen[0] != "" {
		t.Fatalf("the manager saw conversations %q, want exactly an empty one", seen)
	}
}
