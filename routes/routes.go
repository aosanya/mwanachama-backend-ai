package routes

import (
	"github.com/aosanya/mwanachama-backend-shared/dispatch"
	"github.com/aosanya/mwanachama-backend-shared/httpwire"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

type Route = httpwire.Route

type Mount = dispatch.Mount

var Sentinels = map[string]error{
	"ErrAccountNotFound": ai.ErrAccountNotFound,
	"ErrInvalidAccount":  ai.ErrInvalidAccount,
	"ErrDuplicateName":   ai.ErrDuplicateName,
	"ErrInvalidKey":      ai.ErrInvalidKey,
	"ErrNoMasterKey":     ai.ErrNoMasterKey,
	"ErrNoKey":           ai.ErrNoKey,
	"ErrUnsupportedKind": ai.ErrUnsupportedKind,
}

var AnonymousActions = []string{}

var Table = dispatch.NewTable(ai.Operations(), Sentinels, AnonymousActions...)

func Build(am ai.AccountManager) ([]Route, error) { return Table.Build(am, Mount{}) }

func BuildFor(am ai.AccountManager, m Mount) ([]Route, error) { return Table.Build(am, m) }

func Routes(am ai.AccountManager) []Route { return Table.Routes(am, Mount{}) }

func Split(am ai.AccountManager, m Mount) dispatch.Split { return Table.Split(am, m) }
