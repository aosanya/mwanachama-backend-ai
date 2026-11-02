package routes_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	ai "github.com/aosanya/mwanachama-backend-ai"
	"github.com/aosanya/mwanachama-backend-ai/routes"
)

type calledTools struct {
	t  *testing.T
	by map[string]routes.Tool
}

func newCalledTools(t *testing.T) *calledTools {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	pool, _ := db.DB()
	pool.SetMaxOpenConns(1)
	s := mustSpec(t)
	s.Instance = "toolstest"
	if err := ai.Provision(db, s); err != nil {
		t.Fatal(err)
	}
	m, err := ai.NewManager(db, s)
	if err != nil {
		t.Fatal(err)
	}
	tools, err := routes.BuildToolsFor(m, routes.Mount{Caller: func(context.Context) string { return "ann" }})
	if err != nil {
		t.Fatal(err)
	}
	c := &calledTools{t: t, by: map[string]routes.Tool{}}
	for _, tool := range tools {
		c.by[tool.Name] = tool
	}
	return c
}

func (c *calledTools) call(name string, args map[string]any) (string, error) {
	c.t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		c.t.Fatal(err)
	}
	out, err := c.by[name].Invoke(context.Background(), raw)
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		c.t.Fatal(err)
	}
	return string(encoded), nil
}

func (c *calledTools) mustCall(name string, args map[string]any) string {
	c.t.Helper()
	out, err := c.call(name, args)
	if err != nil {
		c.t.Fatalf("%s: %v", name, err)
	}
	return out
}

func (c *calledTools) schemaProperties(name string) map[string]json.RawMessage {
	c.t.Helper()
	tool, ok := c.by[name]
	if !ok {
		c.t.Fatalf("no tool %s", name)
	}
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
		c.t.Fatal(err)
	}
	return schema.Properties
}
