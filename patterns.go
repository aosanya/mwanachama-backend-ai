package ai

import (
	"unicode"
	"unicode/utf8"
)

const maxChatLabelRunes = 60

var patterns = map[string]func(string) bool{
	"chat_label": plainLabel,
}

func plainLabel(s string) bool {
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > maxChatLabelRunes {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
