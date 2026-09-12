package captcha

import (
	"context"
	"errors"
	"testing"
	"time"

	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
)

type fakeEngine struct {
	challenge GeneratedChallenge
	err       error
}

func (f fakeEngine) Generate() (GeneratedChallenge, error) { return f.challenge, f.err }

type fakeStore struct {
	value     *Secret
	setID     string
	setTTL    time.Duration
	takeCalls int
}

func (f *fakeStore) Set(_ context.Context, id string, value Secret, ttl time.Duration) error {
	f.setID, f.setTTL = id, ttl
	f.value = &value
	return nil
}
func (f *fakeStore) Take(_ context.Context, id string) (*Secret, error) {
	f.takeCalls++
	if f.value == nil || id != "captcha-id" {
		return nil, nil
	}
	value := *f.value
	f.value = nil
	return &value, nil
}

type fakeSettings struct {
	values map[string]sharedsetting.Record
}

func (f fakeSettings) FindByKey(_ context.Context, key string) (sharedsetting.Record, error) {
	value, ok := f.values[key]
	if !ok {
		return sharedsetting.Record{}, errors.New("missing")
	}
	return value, nil
}

func TestServiceGenerateAndVerifyConsumesChallenge(t *testing.T) {
	store := &fakeStore{}
	service := NewService(fakeEngine{challenge: GeneratedChallenge{AnswerX: 120, AnswerY: 80}}, store, fakeSettings{values: map[string]sharedsetting.Record{
		sharedsetting.AuthCaptchaTTLKey:          {Key: sharedsetting.AuthCaptchaTTLKey, Value: "2", ValueType: sharedsetting.ValueTypeNumber, IsEnabled: yesno.Yes},
		sharedsetting.AuthCaptchaSlidePaddingKey: {Key: sharedsetting.AuthCaptchaSlidePaddingKey, Value: "3", ValueType: sharedsetting.ValueTypeNumber, IsEnabled: yesno.Yes},
	}}, WithIDGenerator(func() (string, error) { return "captcha-id", nil }))
	challenge, err := service.Generate(context.Background())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if challenge.ID != "captcha-id" || challenge.ExpiresIn != 120 || store.setID != "captcha-id" || store.setTTL != 2*time.Minute {
		t.Fatalf("unexpected challenge %#v", challenge)
	}
	if err := service.Verify(context.Background(), VerifyInput{ID: "captcha-id", X: 122, Y: 81}); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if store.takeCalls != 1 {
		t.Fatalf("Take calls = %d, want 1", store.takeCalls)
	}
	if err := service.Verify(context.Background(), VerifyInput{ID: "captcha-id", X: 120, Y: 80}); err == nil {
		t.Fatal("expected reused challenge to fail")
	}
}

func TestServiceRejectsWrongAnswerAndStoreFailure(t *testing.T) {
	store := &fakeStore{value: &Secret{AnswerX: 120, AnswerY: 80}}
	service := NewService(fakeEngine{}, store, fakeSettings{values: map[string]sharedsetting.Record{
		sharedsetting.AuthCaptchaSlidePaddingKey: {Key: sharedsetting.AuthCaptchaSlidePaddingKey, Value: "3", ValueType: sharedsetting.ValueTypeNumber, IsEnabled: yesno.Yes},
	}}, WithIDGenerator(func() (string, error) { return "captcha-id", nil }))
	if err := service.Verify(context.Background(), VerifyInput{ID: "captcha-id", X: 10, Y: 80}); err == nil {
		t.Fatal("expected wrong answer to fail")
	}
	if err := service.Verify(context.Background(), VerifyInput{ID: "", X: 0, Y: 0}); err == nil {
		t.Fatal("expected missing id to fail")
	}
}
