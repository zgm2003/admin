package recipientRule

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"admin/server/internal/secretkey"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type fakeRepository struct {
	rows      []Model
	findErr   error
	createErr error
	writeErr  error
	created   Model
	updated   Model
	updatedID int64
	statusID  int64
	status    int16
	deletedID int64
}

func (f *fakeRepository) List(context.Context) ([]Model, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	return append([]Model(nil), f.rows...), nil
}

func (f *fakeRepository) FindByID(_ context.Context, id int64) (Model, error) {
	if f.findErr != nil {
		return Model{}, f.findErr
	}
	for _, row := range f.rows {
		if row.ID == id {
			return row, nil
		}
	}
	return Model{}, gorm.ErrRecordNotFound
}

func (f *fakeRepository) Create(_ context.Context, value *Model) error {
	if f.createErr != nil {
		return f.createErr
	}
	if f.writeErr != nil {
		return f.writeErr
	}
	f.created = *value
	value.ID = 11
	return nil
}

func (f *fakeRepository) Update(_ context.Context, value *Model, now time.Time) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	f.updatedID = value.ID
	f.updated = *value
	return nil
}

func (f *fakeRepository) UpdateStatus(_ context.Context, id int64, status int16, now time.Time) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	f.statusID = id
	f.status = status
	return nil
}

func (f *fakeRepository) Delete(_ context.Context, id int64) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	f.deletedID = id
	return nil
}

func testKeys(t *testing.T) *secretkey.KeyRing {
	t.Helper()
	keys, err := secretkey.New(strings.Repeat("s", 64))
	if err != nil {
		t.Fatal(err)
	}
	return keys
}

func appErrorCode(err error) int {
	var appError *apperror.Error
	if errors.As(err, &appError) {
		return appError.Code
	}
	return 0
}

func ruleRow(t *testing.T, keys *secretkey.KeyRing, id int64, scope, pattern, action string, enabled yesno.Value) Model {
	t.Helper()
	ciphertext, _, err := secretkey.EncryptSMSValue(keys.SMSEncryptionKey(), pattern)
	if err != nil {
		t.Fatal(err)
	}
	return Model{
		ID: id, Scope: scope, PatternCiphertext: ciphertext,
		PatternHint: patternHint(scope, pattern), PatternHMAC: patternHMAC(keys, pattern),
		Action: action, Name: "rule", IsEnabled: enabled,
		CreatedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
	}
}

func TestListReturnsHintsOnly(t *testing.T) {
	keys := testKeys(t)
	repository := &fakeRepository{rows: []Model{ruleRow(t, keys, 1, ScopePhone, "+8615671628271", ActionDeny, yesno.Yes)}}
	safes, err := NewService(repository, keys).List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(safes) != 1 || safes[0].PatternHint != "156****8271" {
		t.Fatalf("safes = %+v", safes)
	}
	if strings.Contains(safes[0].PatternHint, "15671628271") {
		t.Fatal("list leaked the full phone pattern")
	}
}

func TestCreateNormalizesEncryptsAndHashesThePattern(t *testing.T) {
	keys := testKeys(t)
	repository := &fakeRepository{}
	service := NewService(repository, keys)

	safe, err := service.Create(context.Background(), CreateInput{
		Scope: ScopePhone, Pattern: " 156 7162 8271 ", Action: ActionDeny, Name: " 黑名单 ", IsEnabled: yesno.Yes,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if repository.created.PatternCiphertext[:7] != "sms:v1:" {
		t.Fatalf("ciphertext = %q", repository.created.PatternCiphertext)
	}
	if repository.created.PatternHint != "156****8271" || repository.created.Name != "黑名单" {
		t.Fatalf("created = %+v", repository.created)
	}
	if repository.created.PatternHMAC != patternHMAC(keys, "+8615671628271") {
		t.Fatal("pattern hmac is not derived from the normalized pattern")
	}
	if safe.PatternHint != "156****8271" || safe.Scope != ScopePhone {
		t.Fatalf("safe = %+v", safe)
	}
}

func TestCreateRejectsPatternsOutsideTheirScope(t *testing.T) {
	keys := testKeys(t)
	for _, test := range []struct{ name, scope, pattern string }{
		{name: "phone scope with prefix", scope: ScopePhone, pattern: "+86156"},
		{name: "phone scope with international number", scope: ScopePhone, pattern: "+14155552671"},
		{name: "prefix scope without country code", scope: ScopePrefix, pattern: "156"},
		{name: "prefix scope too short", scope: ScopePrefix, pattern: "+8615"},
		{name: "prefix scope too long", scope: ScopePrefix, pattern: "+8615671628271"},
		{name: "prefix scope non numeric", scope: ScopePrefix, pattern: "+8615abc"},
		{name: "unknown scope", scope: "domain", pattern: "+86156"},
		{name: "blank pattern", scope: ScopePhone, pattern: "   "},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &fakeRepository{}
			_, err := NewService(repository, keys).Create(context.Background(), CreateInput{
				Scope: test.scope, Pattern: test.pattern, Action: ActionAllow, Name: "rule", IsEnabled: yesno.Yes,
			})
			if appErrorCode(err) != apperror.CodeInvalidRequest {
				t.Fatalf("Create() error = %v, want invalid request", err)
			}
			if repository.created.ID != 0 {
				t.Fatal("invalid pattern reached the repository")
			}
		})
	}
}

func TestCreateMapsUniqueConflictsToConflict(t *testing.T) {
	keys := testKeys(t)
	repository := &fakeRepository{createErr: ErrConflict}
	_, err := NewService(repository, keys).Create(context.Background(), CreateInput{
		Scope: ScopePhone, Pattern: "+8615671628271", Action: ActionDeny, Name: "rule", IsEnabled: yesno.Yes,
	})
	if appErrorCode(err) != apperror.CodeConflict {
		t.Fatalf("Create() error = %v, want conflict", err)
	}
}

func TestUpdateKeepsThePatternWhenItIsOmitted(t *testing.T) {
	keys := testKeys(t)
	row := ruleRow(t, keys, 3, ScopePhone, "+8615671628271", ActionDeny, yesno.Yes)
	repository := &fakeRepository{rows: []Model{row}}

	_, err := NewService(repository, keys).Update(context.Background(), 3, UpdateInput{
		Scope: ScopePhone, Action: ActionAllow, Name: "改名", IsEnabled: yesno.Yes,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if repository.updated.PatternCiphertext != row.PatternCiphertext || repository.updated.PatternHMAC != row.PatternHMAC {
		t.Fatalf("pattern was replaced: %+v", repository.updated)
	}
	if repository.updated.Action != ActionAllow || repository.updated.Name != "改名" {
		t.Fatalf("editable fields were not saved: %+v", repository.updated)
	}
}

func TestUpdateRequiresAPatternWhenTheScopeChanges(t *testing.T) {
	keys := testKeys(t)
	row := ruleRow(t, keys, 3, ScopePhone, "+8615671628271", ActionDeny, yesno.Yes)
	repository := &fakeRepository{rows: []Model{row}}
	_, err := NewService(repository, keys).Update(context.Background(), 3, UpdateInput{
		Scope: ScopePrefix, Action: ActionDeny, Name: "改名", IsEnabled: yesno.Yes,
	})
	if appErrorCode(err) != apperror.CodeInvalidRequest {
		t.Fatalf("Update() error = %v, want invalid request", err)
	}

	pattern := "+86156"
	if _, err := NewService(repository, keys).Update(context.Background(), 3, UpdateInput{
		Scope: ScopePrefix, Pattern: &pattern, Action: ActionDeny, Name: "改名", IsEnabled: yesno.Yes,
	}); err != nil {
		t.Fatalf("Update() with a new pattern error = %v", err)
	}
	if repository.updated.Scope != ScopePrefix || repository.updated.PatternHint != "+86156****" {
		t.Fatalf("updated = %+v", repository.updated)
	}
}

func TestUpdateAndDeleteRequireAnExistingRule(t *testing.T) {
	keys := testKeys(t)
	service := NewService(&fakeRepository{}, keys)
	if _, err := service.Update(context.Background(), 42, UpdateInput{Action: ActionDeny, Name: "n", IsEnabled: yesno.Yes}); appErrorCode(err) != apperror.CodeNotFound {
		t.Fatalf("Update() error = %v, want not found", err)
	}
	if err := service.UpdateStatus(context.Background(), 42, yesno.No); appErrorCode(err) != apperror.CodeNotFound {
		t.Fatalf("UpdateStatus() error = %v, want not found", err)
	}
	if err := service.Delete(context.Background(), 42); appErrorCode(err) != apperror.CodeNotFound {
		t.Fatalf("Delete() error = %v, want not found", err)
	}
}

func TestUpdateStatusAndDeleteForwardToTheRepository(t *testing.T) {
	keys := testKeys(t)
	row := ruleRow(t, keys, 5, ScopePhone, "+8615671628271", ActionDeny, yesno.Yes)
	repository := &fakeRepository{rows: []Model{row}}
	service := NewService(repository, keys)

	if err := service.UpdateStatus(context.Background(), 5, yesno.No); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if err := service.Delete(context.Background(), 5); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if repository.statusID != 5 || repository.status != 0 || repository.deletedID != 5 {
		t.Fatalf("status=%d/%d deleted=%d", repository.statusID, repository.status, repository.deletedID)
	}
}

func TestEvaluatePrefersExactThenLongestPrefixAndDeny(t *testing.T) {
	keys := testKeys(t)
	repository := &fakeRepository{rows: []Model{
		ruleRow(t, keys, 1, ScopePrefix, "+86156", ActionAllow, yesno.Yes),
		ruleRow(t, keys, 2, ScopePrefix, "+86156", ActionDeny, yesno.Yes),
		ruleRow(t, keys, 3, ScopePrefix, "+861560", ActionAllow, yesno.Yes),
		ruleRow(t, keys, 4, ScopePhone, "+8613800000000", ActionAllow, yesno.Yes),
		ruleRow(t, keys, 5, ScopePrefix, "+86139", ActionAllow, yesno.No),
	}}
	service := NewService(repository, keys)

	// The longest matching prefix wins even when a shorter prefix denies.
	longest, err := service.Evaluate(context.Background(), "+8615600000000")
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if !longest.Allowed || longest.RuleID != 3 || longest.Reason != ReasonPrefixMatch {
		t.Fatalf("longest prefix decision = %+v", longest)
	}

	// At the same prefix length deny outranks allow.
	sameLength, err := service.Evaluate(context.Background(), "+8615699999999")
	if err != nil {
		t.Fatal(err)
	}
	if sameLength.Allowed || sameLength.RuleID != 2 {
		t.Fatalf("same length decision = %+v", sameLength)
	}

	// A full number rule outranks every prefix rule.
	exact, err := service.Evaluate(context.Background(), "+8613800000000")
	if err != nil {
		t.Fatal(err)
	}
	if !exact.Allowed || exact.RuleID != 4 || exact.Reason != ReasonPhoneExact {
		t.Fatalf("exact decision = %+v", exact)
	}

	// A disabled allow rule cannot match, so the default allow applies.
	disabled, err := service.Evaluate(context.Background(), "+8613912345678")
	if err != nil {
		t.Fatal(err)
	}
	if !disabled.Allowed || disabled.RuleID != 0 || disabled.Reason != ReasonDefaultAllow {
		t.Fatalf("disabled rule decision = %+v", disabled)
	}
}

func TestEvaluateDefaultsToAllowWithoutAMatch(t *testing.T) {
	keys := testKeys(t)
	repository := &fakeRepository{rows: []Model{
		ruleRow(t, keys, 1, ScopePrefix, "+86156", ActionDeny, yesno.Yes),
	}}
	decision, err := NewService(repository, keys).Evaluate(context.Background(), "+8613912345678")
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Allowed || decision.RuleID != 0 || decision.Reason != ReasonDefaultAllow {
		t.Fatalf("decision = %+v", decision)
	}
}

func TestEvaluateFailsClosedWhenAPatternCannotBeDecrypted(t *testing.T) {
	keys := testKeys(t)
	repository := &fakeRepository{rows: []Model{{
		ID: 1, Scope: ScopePhone, PatternCiphertext: "sms:v1:corrupted", PatternHMAC: "hmac",
		Action: ActionDeny, Name: "rule", IsEnabled: yesno.Yes,
	}}}
	_, err := NewService(repository, keys).Evaluate(context.Background(), "+8615671628271")
	if appErrorCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("Evaluate() error = %v, want dependency unavailable", err)
	}
}
