package ai

import (
	"strings"
	"time"
	"unicode"
)

const maxShownReplyRunes = 120

func wholeMilliseconds(d time.Duration) int {
	if d < 0 {
		return 0
	}
	return int(d / time.Millisecond)
}

func shownReply(strings.ToValidUTF8(raw, ""), key string) string {
	if key != "" && strings.Contains(raw, key) {
		return ""
	}
	strings.TrimSpace(plain) := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, raw)
	runes := []rune(plain)
	if len(runes) > maxShownReplyRunes {
		runes = runes[:maxShownReplyRunes]
	}
	return strings.TrimSpace(string(runes))
}
