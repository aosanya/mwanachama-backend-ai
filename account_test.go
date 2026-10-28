package ai_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"testing"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

var ctx = context.Background()

func mustAdd(t *testing.T, m ai.Manager, agency, name string) ai.Account {
	t.Helper()
	a, err := m.CreateAccount(ctx, agency, validAccount(name))
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	return a
}

func TestAddChangeListRemove(t *testing.T) {
	m, _, _ := newTestManager(t)
	a := mustAdd(t, m, "ag-1", "primary")
	if a.ID == "" || a.KeySet {
		t.Fatalf("unexpected account %+v", a)
	}
	in := validAccount("renamed")
	in.TokensPerDay = ip(5000)
	in.Status = "disabled"
	up, err := m.UpdateAccount(ctx, "ag-1", a.ID, in)
	if err != nil || up.Name != "renamed" || iv(up.TokensPerDay) != 5000 || up.Status != "disabled" {
		t.Fatalf("update: %+v %v", up, err)
	}
	list, _ := m.ListAccounts(ctx, "ag-1")
	if len(list) != 1 {
		t.Fatalf("list = %d", len(list))
	}
	if _, err := m.DeleteAccount(ctx, "ag-1", a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := m.GetAccount(ctx, "ag-1", a.ID); !errors.Is(err, ai.ErrAccountNotFound) {
		t.Fatalf("after delete: %v", err)
	}
}

func TestNameIsUniquePerAgencyOnly(t *testing.T) {
	m, _, _ := newTestManager(t)
	mustAdd(t, m, "ag-1", "main")
	if _, err := m.CreateAccount(ctx, "ag-1", validAccount("main")); !errors.Is(err, ai.ErrDuplicateName) {
		t.Fatalf("same agency: %v", err)
	}
	if _, err := m.CreateAccount(ctx, "ag-2", validAccount("main")); err != nil {
		t.Fatalf("other agency: %v", err)
	}
	other := mustAdd(t, m, "ag-1", "second")
	if _, err := m.UpdateAccount(ctx, "ag-1", other.ID, validAccount("main")); !errors.Is(err, ai.ErrDuplicateName) {
		t.Fatalf("rename onto taken: %v", err)
	}
}

func TestProviderFieldsAreRequiredAndNoDefaultIsFilled(t *testing.T) {
	m, _, _ := newTestManager(t)
	for _, mut := range []func(*ai.Account){
		func(a *ai.Account) { a.Provider = "" },
		func(a *ai.Account) { a.BaseURL = "" },
		func(a *ai.Account) { a.ModelName = "" },
		func(a *ai.Account) { a.Status = "" },
		func(a *ai.Account) { a.Name = "" },
	} {
		a := validAccount("x")
		mut(&a)
		if _, err := m.CreateAccount(ctx, "ag-1", a); !errors.Is(err, ai.ErrInvalidAccount) {
			t.Errorf("want invalid, got %v", err)
		}
	}
}

func TestKindWithoutAnAdapterIsRefusedOnCreateAndUpdate(t *testing.T) {
	m, _, _ := newTestManager(t)
	a := validAccount("x")
	a.Provider = "unknown_kind"
	if _, err := m.CreateAccount(ctx, "ag-1", a); !errors.Is(err, ai.ErrUnsupportedKind) {
		t.Fatalf("create: %v", err)
	}
	ok := mustAdd(t, m, "ag-1", "ok")
	if _, err := m.UpdateAccount(ctx, "ag-1", ok.ID, a); !errors.Is(err, ai.ErrUnsupportedKind) {
		t.Fatalf("update: %v", err)
	}
}

func TestNegativeLimitIsRefused(t *testing.T) {
	m, _, _ := newTestManager(t)
	a := validAccount("x")
	a.RequestsPerMinute = ip(-1)
	if _, err := m.CreateAccount(ctx, "ag-1", a); !errors.Is(err, ai.ErrInvalidAccount) {
		t.Fatalf("got %v", err)
	}
}

func TestOneAgencyCannotSeeAnothersAccounts(t *testing.T) {
	m, _, _ := newTestManager(t, ai.WithMasterKey(master))
	a := mustAdd(t, m, "ag-1", "main")
	if list, _ := m.ListAccounts(ctx, "ag-2"); len(list) != 0 {
		t.Fatalf("ag-2 sees %d", len(list))
	}
	if _, err := m.GetAccount(ctx, "ag-2", a.ID); !errors.Is(err, ai.ErrAccountNotFound) {
		t.Fatalf("get: %v", err)
	}
	if _, err := m.UpdateAccount(ctx, "ag-2", a.ID, validAccount("x")); !errors.Is(err, ai.ErrAccountNotFound) {
		t.Fatalf("update: %v", err)
	}
	if _, err := m.SetAccountKey(ctx, "ag-2", a.ID, ai.KeyInput{Key: syntheticKey}); !errors.Is(err, ai.ErrAccountNotFound) {
		t.Fatalf("set key: %v", err)
	}
	if _, err := m.DeleteAccount(ctx, "ag-2", a.ID); !errors.Is(err, ai.ErrAccountNotFound) {
		t.Fatalf("delete: %v", err)
	}
	if _, err := m.GetAccount(ctx, "ag-1", a.ID); err != nil {
		t.Fatalf("owner lost it: %v", err)
	}
	m.SetAccountKey(ctx, "ag-1", a.ID, ai.KeyInput{Key: syntheticKey})
	if _, err := m.AccountKey(ctx, "ag-2", a.ID); !errors.Is(err, ai.ErrAccountNotFound) {
		t.Fatalf("read key: %v", err)
	}
}

func TestWithoutAMasterKeyNoKeyIsStored(t *testing.T) {
	m, db, s := newTestManager(t)
	a := mustAdd(t, m, "ag-1", "main")
	if _, err := m.SetAccountKey(ctx, "ag-1", a.ID, ai.KeyInput{Key: syntheticKey}); !errors.Is(err, ai.ErrNoMasterKey) {
		t.Fatalf("got %v", err)
	}
	o, _ := s.ByRole("account_key")
	var n int64
	db.Table(s.TableFor(o)).Count(&n)
	if n != 0 {
		t.Fatalf("%d key rows stored", n)
	}
	got, _ := m.GetAccount(ctx, "ag-1", a.ID)
	if got.KeySet {
		t.Fatal("key_set true without a key")
	}
}

func TestKeyIsSealedAtRestAndReadableOnlyThroughTheGoMethod(t *testing.T) {
	m, db, s := newTestManager(t, ai.WithMasterKey(master))
	a := mustAdd(t, m, "ag-1", "main")
	set, err := m.SetAccountKey(ctx, "ag-1", a.ID, ai.KeyInput{Key: syntheticKey})
	if err != nil || !set.KeySet {
		t.Fatalf("set: %+v %v", set, err)
	}
	o, _ := s.ByRole("account_key")
	var sealed string
	db.Table(s.TableFor(o)).Select("sealed").Row().Scan(&sealed)
	if sealed == "" || strings.Contains(sealed, syntheticKey) {
		t.Fatalf("stored value %q is not sealed", sealed)
	}
	got, err := m.AccountKey(ctx, "ag-1", a.ID)
	if err != nil || got != syntheticKey {
		t.Fatalf("AccountKey = %q, %v", got, err)
	}
	if _, err := m.SetAccountKey(ctx, "ag-1", a.ID, ai.KeyInput{Key: "sk-REPLACEMENT-1"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := m.AccountKey(ctx, "ag-1", a.ID); got != "sk-REPLACEMENT-1" {
		t.Fatalf("replace: %q", got)
	}
	other, _ := ai.NewManager(db, s, ai.WithMasterKey([]byte("ffffffffffffffffffffffffffffffff")))
	if v, err := other.AccountKey(ctx, "ag-1", a.ID); err == nil || v != "" {
		t.Fatalf("wrong master opened the key: %q %v", v, err)
	}
}

func TestEmptyKeyIsRefused(t *testing.T) {
	m, _, _ := newTestManager(t, ai.WithMasterKey(master))
	a := mustAdd(t, m, "ag-1", "main")
	if _, err := m.SetAccountKey(ctx, "ag-1", a.ID, ai.KeyInput{Key: "  "}); !errors.Is(err, ai.ErrInvalidKey) {
		t.Fatalf("got %v", err)
	}
}

func TestKeyNeverAppearsInAnyOutputErrorOrLog(t *testing.T) {
	var logs bytes.Buffer
	log.SetOutput(&logs)
	defer log.SetOutput(nil)
	echo := &fakeProber{out: ai.ProbeOK, reply: "OK " + syntheticKey}
	m, _, _ := newTestManager(t, ai.WithMasterKey(master), ai.WithProber(echo))
	a := mustAdd(t, m, "ag-1", "main")
	var seen []string
	note := func(v any, err error) {
		b, _ := json.Marshal(v)
		seen = append(seen, string(b))
		if err != nil {
			seen = append(seen, err.Error())
		}
	}
	set, err := m.SetAccountKey(ctx, "ag-1", a.ID, ai.KeyInput{Key: syntheticKey})
	note(set, err)
	tested, err := m.TestAccount(ctx, "ag-1", a.ID, ai.KeyInput{})
	note(tested, err)
	g, err := m.GetAccount(ctx, "ag-1", a.ID)
	note(g, err)
	l, err := m.ListAccounts(ctx, "ag-1")
	note(l, err)
	u, err := m.UpdateAccount(ctx, "ag-1", a.ID, validAccount("again"))
	note(u, err)
	_, err = m.SetAccountKey(ctx, "ag-2", a.ID, ai.KeyInput{Key: syntheticKey})
	note(nil, err)
	_, err = m.SetAccountKey(ctx, "ag-1", "missing", ai.KeyInput{Key: syntheticKey})
	note(nil, err)
	d, err := m.DeleteAccount(ctx, "ag-1", a.ID)
	note(d, err)
	seen = append(seen, logs.String())
	for _, s := range seen {
		if strings.Contains(s, syntheticKey) || strings.Contains(strings.ToLower(s), `"key"`) {
			t.Fatalf("key leaked in %q", s)
		}
	}
}

func TestSetAccountLimitsChangesOnlyLimits(t *testing.T) {
	m, _, _ := newTestManager(t, ai.WithMasterKey(master))
	ctx := context.Background()
	made, err := m.CreateAccount(ctx, "ag-1", validAccount("main"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.SetAccountKey(ctx, "ag-1", made.ID, ai.KeyInput{Key: syntheticKey}); err != nil {
		t.Fatal(err)
	}
	got, err := m.SetAccountLimits(ctx, "ag-1", made.ID, ai.Limits{TokensPerCall: ip(1), TokensPerDay: ip(2), TokensPerMonth: ip(3), RequestsPerDay: ip(4), RequestsPerMinute: ip(5)})
	if err != nil {
		t.Fatal(err)
	}
	read, err := m.GetAccount(ctx, "ag-1", made.ID)
	if err != nil {
		t.Fatal(err)
	}
	if iv(read.TokensPerCall) != 1 || iv(read.TokensPerDay) != 2 || iv(read.TokensPerMonth) != 3 || iv(read.RequestsPerDay) != 4 || iv(read.RequestsPerMinute) != 5 {
		t.Fatalf("limits not stored: %+v", read)
	}
	if read.Name != made.Name || read.Provider != made.Provider || read.BaseURL != made.BaseURL || read.ModelName != made.ModelName || read.Status != made.Status || !read.KeySet {
		t.Fatalf("something besides limits changed: %+v", read)
	}
	if got.ID != made.ID {
		t.Fatalf("returned %+v", got)
	}
	key, err := m.AccountKey(ctx, "ag-1", made.ID)
	if err != nil || key != syntheticKey {
		t.Fatalf("key changed: %q %v", key, err)
	}
	if _, err := m.SetAccountLimits(ctx, "ag-1", made.ID, ai.Limits{TokensPerDay: ip(-1)}); !errors.Is(err, ai.ErrInvalidAccount) {
		t.Fatalf("negative: %v", err)
	}
	if _, err := m.SetAccountLimits(ctx, "ag-2", made.ID, ai.Limits{TokensPerDay: ip(9)}); !errors.Is(err, ai.ErrAccountNotFound) {
		t.Fatalf("other agency: %v", err)
	}
	again, _ := m.GetAccount(ctx, "ag-1", made.ID)
	if iv(again.TokensPerDay) != 2 {
		t.Fatalf("refused call changed limits: %+v", again)
	}
}

func TestReplacingTheKeyClearsTheLastTestResult(t *testing.T) {
	f := &fakeProber{out: ai.ProbeRefused}
	m, _, _ := newTestManager(t, ai.WithMasterKey(master), ai.WithProber(f))
	a := mustAdd(t, m, "ag-1", "main")
	if _, err := m.SetAccountKey(ctx, "ag-1", a.ID, ai.KeyInput{Key: "sk-OLD-1"}); err != nil {
		t.Fatal(err)
	}
	tested, err := m.TestAccount(ctx, "ag-1", a.ID, ai.KeyInput{})
	if err != nil || tested.LastTestResult != ai.TestRefused || tested.LastTestedAt == "" {
		t.Fatalf("test: %+v %v", tested, err)
	}
	replaced, err := m.SetAccountKey(ctx, "ag-1", a.ID, ai.KeyInput{Key: "sk-NEW-2"})
	if err != nil {
		t.Fatal(err)
	}
	if replaced.LastTestResult != "" || replaced.LastTestedAt != "" {
		t.Fatalf("answer still carries the old test: %+v", replaced)
	}
	held, _ := m.GetAccount(ctx, "ag-1", a.ID)
	if held.LastTestResult != "" || held.LastTestedAt != "" || !held.KeySet {
		t.Fatalf("stored account still carries the old test: %+v", held)
	}
}

func TestRefusedKeyReplacementKeepsTheLastTestResult(t *testing.T) {
	f := &fakeProber{out: ai.ProbeOK}
	m, _, _ := newTestManager(t, ai.WithMasterKey(master), ai.WithProber(f))
	a := mustAdd(t, m, "ag-1", "main")
	m.SetAccountKey(ctx, "ag-1", a.ID, ai.KeyInput{Key: "sk-OLD-1"})
	m.TestAccount(ctx, "ag-1", a.ID, ai.KeyInput{})
	if _, err := m.SetAccountKey(ctx, "ag-1", a.ID, ai.KeyInput{Key: " "}); !errors.Is(err, ai.ErrInvalidKey) {
		t.Fatalf("got %v", err)
	}
	held, _ := m.GetAccount(ctx, "ag-1", a.ID)
	if held.LastTestResult != ai.TestWorking || held.LastTestedAt == "" {
		t.Fatalf("a refused key wiped the test: %+v", held)
	}
}

func onlyHost(host string) ai.Option {
	return ai.WithBaseURLPolicy(func(base string) error {
		if !strings.HasPrefix(base, "https://"+host+"/") {
			return errors.New("this address is not allowed")
		}
		return nil
	})
}

func TestBaseURLPolicyRefusesAnAddressOnCreate(t *testing.T) {
	m, _, _ := newTestManager(t, onlyHost("models.example.test"))
	bad := validAccount("x")
	bad.BaseURL = "https://elsewhere.example.test/v1"
	if _, err := m.CreateAccount(ctx, "ag-1", bad); !errors.Is(err, ai.ErrInvalidAccount) {
		t.Fatalf("got %v", err)
	}
	if list, _ := m.ListAccounts(ctx, "ag-1"); len(list) != 0 {
		t.Fatalf("a refused account was stored: %+v", list)
	}
	if _, err := m.CreateAccount(ctx, "ag-1", validAccount("ok")); err != nil {
		t.Fatalf("allowed address: %v", err)
	}
}

func TestBaseURLPolicyRefusesAnAddressOnUpdate(t *testing.T) {
	m, _, _ := newTestManager(t, onlyHost("models.example.test"))
	a := mustAdd(t, m, "ag-1", "main")
	bad := validAccount("main")
	bad.BaseURL = "http://169.254.169.254/latest"
	if _, err := m.UpdateAccount(ctx, "ag-1", a.ID, bad); !errors.Is(err, ai.ErrInvalidAccount) {
		t.Fatalf("got %v", err)
	}
	held, _ := m.GetAccount(ctx, "ag-1", a.ID)
	if held.BaseURL != "https://models.example.test/v1" {
		t.Fatalf("a refused update changed the address: %q", held.BaseURL)
	}
}

func TestBaseURLPolicyAnswerNamesTheReasonAndNotTheKey(t *testing.T) {
	m, _, _ := newTestManager(t, ai.WithMasterKey(master), onlyHost("models.example.test"))
	bad := validAccount("x")
	bad.BaseURL = "https://elsewhere.example.test/v1"
	_, err := m.CreateAccount(ctx, "ag-1", bad)
	if err == nil || !strings.Contains(err.Error(), "this address is not allowed") || strings.Contains(err.Error(), syntheticKey) {
		t.Fatalf("got %v", err)
	}
}

func TestWithoutABaseURLPolicyAnyAddressIsAccepted(t *testing.T) {
	m, _, _ := newTestManager(t)
	a := validAccount("x")
	a.BaseURL = "http://localhost:9/v1"
	if _, err := m.CreateAccount(ctx, "ag-1", a); err != nil {
		t.Fatalf("got %v", err)
	}
}

func TestChangingLimitsIsRefusedWhileTheStoredAddressIsNotAllowed(t *testing.T) {
	db := openDB(t)
	s := testSpec(t)
	if err := ai.Provision(db, s); err != nil {
		t.Fatal(err)
	}
	open, _ := ai.NewManager(db, s)
	a, err := open.CreateAccount(ctx, "ag-1", validAccount("main"))
	if err != nil {
		t.Fatal(err)
	}
	strict, _ := ai.NewManager(db, s, onlyHost("other.example.test"))
	if _, err := strict.SetAccountLimits(ctx, "ag-1", a.ID, ai.Limits{TokensPerDay: ip(10)}); !errors.Is(err, ai.ErrInvalidAccount) {
		t.Fatalf("got %v", err)
	}
}
