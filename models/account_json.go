package models

import (
	"bytes"
	"encoding/json"
)

func (a *Account) UnmarshalJSON(raw []byte) error {
	type plain Account
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode((*plain)(a)); err != nil {
		return err
	}
	var sent map[string]json.RawMessage
	if err := json.Unmarshal(raw, &sent); err != nil {
		return err
	}
	_, hasEnabled := sent["chat_enabled"]
	_, hasLabel := sent["chat_label"]
	a.chatEnabledOmitted, a.chatLabelOmitted = !hasEnabled, !hasLabel
	return nil
}

func (a Account) WithChatOf(sent Account) Account {
	if !sent.chatEnabledOmitted {
		a.ChatEnabled = sent.ChatEnabled
	}
	if !sent.chatLabelOmitted {
		a.ChatLabel = sent.ChatLabel
	}
	return a
}
