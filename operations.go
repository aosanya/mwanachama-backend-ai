package ai

import _ "embed"

//go:embed ai.operations.json
var operationsJSON []byte

func Operations() []byte { return operationsJSON }
