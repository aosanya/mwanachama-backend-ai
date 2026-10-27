package ai

import "errors"

var (
	ErrAccountNotFound = errors.New("account not found")
	ErrInvalidAccount  = errors.New("invalid account")
	ErrDuplicateName   = errors.New("an account with this name already exists")
	ErrInvalidKey      = errors.New("invalid key")
	ErrNoMasterKey     = errors.New("no master key is configured, so no key can be stored")
	ErrNoKey           = errors.New("this account has no key")
	ErrUnsupportedKind = errors.New("no adapter exists for this provider kind")
)
