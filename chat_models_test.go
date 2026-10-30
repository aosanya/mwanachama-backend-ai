package ai_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/spec"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

func chatModel(t *testing.T, m ai.Manager, agency, name, modelName, label string, on bool) ai.Account {
	t.Helper()
	a := validAccount(name)
	a.ModelName, a.ChatLabel, a.ChatEnabled = modelName, label, on
	made, err := m.CreateAccount(ctx, agency, a)
	if err != nil {
		t.Fatalf("CreateAccount(%s): %v", name, err)
	}
	return made
}

func mustObject(t *testing.T, s *spec.Spec, role string) spec.Object {
	t.Helper()
	o, ok := s.ByRole(role)
	if !ok {
		t.Fatalf("no object with role %s", role)
	}
	return o
}

func labelsOf(items []ai.ChatModel) string {
	var out []string
	for _, it := range items {
		mark := ""
		if it.Picked {
			mark = "*"
		}
		out = append(out, it.Label+mark)
	}
	return strings.Join(out, ",")
}

func TestTheChatListsOnlyEnabledModelsTheAdminAllowedUnderTheirLabelsInTheModelsOrder(t *testing.T) {
	m, _, _ := newTestManager(t)
	chatModel(t, m, "ag", "muse", "muse-spark-1.3", "", true)
	chatModel(t, m, "ag", "hidden", "hidden-model", "Hidden", false)
	chatModel(t, m, "ag", "glm-hf", "GLM-5.3", "GLM-5.3 (Hugging Face)", true)
	off := chatModel(t, m, "ag", "glm-bt", "GLM-5.3", "GLM-5.3 (Baseten)", true)
	off.Status = "disabled"
	if _, err := m.UpdateAccount(ctx, "ag", off.ID, off); err != nil {
		t.Fatal(err)
	}
	chatModel(t, m, "other", "theirs", "theirs", "Theirs", true)

	items, err := m.ListChatModels(ctx, "ag", "ann")
	if err != nil {
		t.Fatal(err)
	}
	if got := labelsOf(items); got != "muse-spark-1.3*,GLM-5.3 (Hugging Face)" {
		t.Fatalf("the picker shows %q", got)
	}
}

func TestTheSameModelFromTwoProvidersIsToldApartByItsLabel(t *testing.T) {
	m, _, _ := newTestManager(t)
	chatModel(t, m, "ag", "hf", "GLM-5.3", "GLM-5.3 (Hugging Face)", true)
	chatModel(t, m, "ag", "bt", "GLM-5.3", "GLM-5.3 (Baseten)", true)
	items, _ := m.ListChatModels(ctx, "ag", "ann")
	if got := labelsOf(items); got != "GLM-5.3 (Hugging Face)*,GLM-5.3 (Baseten)" {
		t.Fatalf("the picker shows %q", got)
	}
}

func TestAHubWithNoModelForTheChatListsNothingAsAnEmptyList(t *testing.T) {
	m, _, _ := newTestManager(t)
	chatModel(t, m, "ag", "hidden", "hidden-model", "", false)
	items, err := m.ListChatModels(ctx, "ag", "ann")
	if err != nil || items == nil || len(items) != 0 {
		t.Fatalf("items = %#v, %v; want an empty, non-nil list", items, err)
	}
	raw, _ := json.Marshal(items)
	if string(raw) != "[]" {
		t.Fatalf("encoded as %s", raw)
	}
}

func TestAPersonsPickIsTheirsAndTheOthersStayOnTheFirstModel(t *testing.T) {
	m, _, _ := newTestManager(t)
	chatModel(t, m, "ag", "one", "one", "One", true)
	two := chatModel(t, m, "ag", "two", "two", "Two", true)

	picked, err := m.PickChatModel(ctx, "ag", "ann", ai.ChatPickInput{AccountID: two.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got := labelsOf(picked); got != "One,Two*" {
		t.Fatalf("after the pick the answer shows %q", got)
	}
	ann, _ := m.ListChatModels(ctx, "ag", "ann")
	bob, _ := m.ListChatModels(ctx, "ag", "bob")
	if labelsOf(ann) != "One,Two*" || labelsOf(bob) != "One*,Two" {
		t.Fatalf("ann sees %q and bob sees %q", labelsOf(ann), labelsOf(bob))
	}
}

func TestAPickSurvivesANewManagerOverTheSameDatabase(t *testing.T) {
	m, db, s := newTestManager(t)
	chatModel(t, m, "ag", "one", "one", "One", true)
	two := chatModel(t, m, "ag", "two", "two", "Two", true)
	if _, err := m.PickChatModel(ctx, "ag", "ann", ai.ChatPickInput{AccountID: two.ID}); err != nil {
		t.Fatal(err)
	}
	again, err := ai.NewManager(db, s)
	if err != nil {
		t.Fatal(err)
	}
	items, _ := again.ListChatModels(ctx, "ag", "ann")
	if labelsOf(items) != "One,Two*" {
		t.Fatalf("the pick was not kept: %q", labelsOf(items))
	}
}

func TestPickingAgainMovesThePickAndLeavesOneRowPerPerson(t *testing.T) {
	m, db, s := newTestManager(t)
	one := chatModel(t, m, "ag", "one", "one", "One", true)
	two := chatModel(t, m, "ag", "two", "two", "Two", true)
	for _, id := range []string{two.ID, one.ID, two.ID} {
		if _, err := m.PickChatModel(ctx, "ag", "ann", ai.ChatPickInput{AccountID: id}); err != nil {
			t.Fatal(err)
		}
	}
	var n int64
	if err := db.Table(s.TableFor(mustObject(t, s, "chat_pick"))).Count(&n).Error; err != nil || n != 1 {
		t.Fatalf("rows = %d, %v", n, err)
	}
}

func TestAModelTheChatMayNotUseCannotBePickedAndAnotherAgencysCannotEither(t *testing.T) {
	m, _, _ := newTestManager(t)
	chatModel(t, m, "ag", "one", "one", "One", true)
	hidden := chatModel(t, m, "ag", "hidden", "hidden", "", false)
	theirs := chatModel(t, m, "other", "theirs", "theirs", "", true)
	if _, err := m.PickChatModel(ctx, "ag", "ann", ai.ChatPickInput{AccountID: hidden.ID}); !errors.Is(err, ai.ErrChatModelUnavailable) {
		t.Fatalf("a model the chat may not use: %v", err)
	}
	if _, err := m.PickChatModel(ctx, "ag", "ann", ai.ChatPickInput{AccountID: theirs.ID}); !errors.Is(err, ai.ErrAccountNotFound) {
		t.Fatalf("another agency's model: %v", err)
	}
	if _, err := m.PickChatModel(ctx, "ag", "ann", ai.ChatPickInput{AccountID: "no-such"}); !errors.Is(err, ai.ErrAccountNotFound) {
		t.Fatalf("a missing model: %v", err)
	}
	if _, err := m.PickChatModel(ctx, "ag", "", ai.ChatPickInput{AccountID: "x"}); !errors.Is(err, ai.ErrInvalidAccount) {
		t.Fatalf("no person: %v", err)
	}
	items, _ := m.ListChatModels(ctx, "ag", "ann")
	if labelsOf(items) != "One*" {
		t.Fatalf("a refused pick changed the list: %q", labelsOf(items))
	}
}

func TestAPickOfAModelThatStoppedBeingAllowedFallsBackToTheFirst(t *testing.T) {
	m, _, _ := newTestManager(t)
	chatModel(t, m, "ag", "one", "one", "One", true)
	two := chatModel(t, m, "ag", "two", "two", "Two", true)
	if _, err := m.PickChatModel(ctx, "ag", "ann", ai.ChatPickInput{AccountID: two.ID}); err != nil {
		t.Fatal(err)
	}
	two.ChatEnabled = false
	if _, err := m.UpdateAccount(ctx, "ag", two.ID, two); err != nil {
		t.Fatal(err)
	}
	items, _ := m.ListChatModels(ctx, "ag", "ann")
	if labelsOf(items) != "One*" {
		t.Fatalf("the picker shows %q", labelsOf(items))
	}
}

func TestRemovingAModelRemovesThePicksOfIt(t *testing.T) {
	m, db, s := newTestManager(t)
	chatModel(t, m, "ag", "one", "one", "One", true)
	two := chatModel(t, m, "ag", "two", "two", "Two", true)
	if _, err := m.PickChatModel(ctx, "ag", "ann", ai.ChatPickInput{AccountID: two.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.DeleteAccount(ctx, "ag", two.ID); err != nil {
		t.Fatal(err)
	}
	var n int64
	if err := db.Table(s.TableFor(mustObject(t, s, "chat_pick"))).Count(&n).Error; err != nil || n != 0 {
		t.Fatalf("rows = %d, %v", n, err)
	}
	items, _ := m.ListChatModels(ctx, "ag", "ann")
	if labelsOf(items) != "One*" {
		t.Fatalf("the picker shows %q", labelsOf(items))
	}
}

func TestTheChatListCarriesNeitherAKeyNorAnythingBeyondTheLabel(t *testing.T) {
	m, _, _ := newTestManager(t, ai.WithMasterKey(master))
	a := chatModel(t, m, "ag", "one", "one", "One", true)
	if _, err := m.SetAccountKey(ctx, "ag", a.ID, ai.KeyInput{Key: syntheticKey}); err != nil {
		t.Fatal(err)
	}
	items, _ := m.ListChatModels(ctx, "ag", "ann")
	raw, _ := json.Marshal(items)
	if strings.Contains(string(raw), syntheticKey) || strings.Contains(string(raw), "key") || strings.Contains(string(raw), "base_url") {
		t.Fatalf("the list shows more than the label: %s", raw)
	}
}
