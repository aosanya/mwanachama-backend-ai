package routes

import (
	"fmt"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

type Tool = dispatch.Tool

func BuildTools(am ai.AccountManager) ([]Tool, error) { return BuildToolsFor(am, Mount{}) }

func BuildToolsFor(am ai.AccountManager, m Mount) ([]Tool, error) {
	s, err := Table.Spec()
	if err != nil {
		return nil, err
	}
	return dispatch.Tools(s, dispatch.Deps{
		Manager: am, Errors: Sentinels, Authorize: m.Authorize, Caller: m.Caller,
	})
}

func Tools(am ai.AccountManager) []Tool { return ToolsFor(am, Mount{}) }

func ToolsFor(am ai.AccountManager, m Mount) []Tool {
	out, err := BuildToolsFor(am, m)
	if err != nil {
		panic(fmt.Sprintf("ai tools: %v", err))
	}
	return out
}
