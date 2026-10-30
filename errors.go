package ai

import "errors"

var (
	ErrAccountNotFound   = errors.New("account not found")
	ErrInvalidAccount    = errors.New("invalid account")
	ErrDuplicateName     = errors.New("an account with this name already exists")
	ErrInvalidKey        = errors.New("invalid key")
	ErrNoMasterKey       = errors.New("no master key is configured, so no key can be stored")
	ErrNoKey             = errors.New("this account has no key")
	ErrNoDefaultAccount  = errors.New("Mark a default AI account under Models before running a test")
	ErrUnsupportedKind   = errors.New("no adapter exists for this provider kind")
	ErrRoleNotFound      = errors.New("AI role not found")
	ErrInvalidRole       = errors.New("invalid AI role")
	ErrDuplicateRoleName = errors.New("an AI role with this name already exists")
	ErrRevisionNotFound  = errors.New("revision not found")
	ErrInvalidModelList  = errors.New("invalid model list")

	ErrChatModelUnavailable = errors.New("this model cannot be used in the chat")

	ErrInvalidChat     = errors.New("invalid chat request")
	ErrChatUnsupported = errors.New("this provider cannot chat with tools yet")
	ErrAccountOff      = errors.New("this account is turned off")
	ErrLimitReached    = errors.New("this account is at its limit")
	ErrModelRefused    = errors.New(TestRefused)
	ErrModelUnreached  = errors.New(TestUnreached)
	ErrModelError      = errors.New(TestModelError)
)
