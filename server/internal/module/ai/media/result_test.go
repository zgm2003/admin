package media

import (
	providerrequest "admin/server/internal/module/ai/providerRequest"
	airun "admin/server/internal/module/ai/run"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type resultCipher struct{ fail bool }

func (c resultCipher) Open(string, string) (string, error) { return "", nil }
func (c resultCipher) Seal(string) (string, string, error) {
	if c.fail {
		return "", "", errors.New("cipher offline")
	}
	return "encrypted-value", "key-version", nil
}
func TestPrepareResultFileHasNoPlaintextSourceAndNeedsNoIO(t *testing.T) {
	now := time.Unix(10, 0).UTC()
	source := "https://media.example/secret?token=private"
	s := &Service{cipher: resultCipher{}}
	in := InlineImport{RunID: 1, ProviderID: 2, Ordinal: 0, RequestSeed: "2:0", MIME: "image/png", TransferTimeout: 900 * time.Second}
	f, err := s.PrepareResultFile(in, source, []string{"media.example"}, now)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(f)
	if strings.Contains(string(raw), source) || strings.Contains(string(raw), "token=private") || f.SourceCiphertext == nil || *f.SourceCiphertext != "encrypted-value" || f.TransferStatus != TransferQueued || f.SourceRecoverable != 1 || !f.TransferDeadline.Equal(now.Add(900*time.Second)) {
		t.Fatalf("fact=%s err=%v", raw, err)
	}
}
func TestPrepareResultFileRetainsExplicitFailureWhenSourceCannotBeEncrypted(t *testing.T) {
	s := &Service{cipher: resultCipher{fail: true}}
	f, err := s.PrepareResultFile(InlineImport{RunID: 1, ProviderID: 2, Ordinal: 0, RequestSeed: "2:0", TransferTimeout: time.Minute}, "https://media.example/secret", nil, time.Now())
	if err != nil || f.TransferStatus != TransferFailed || f.SourceRecoverable != 0 || f.SourceCiphertext != nil || f.ErrorCode == nil {
		t.Fatalf("fact=%+v err=%v", f, err)
	}
}

func TestImageResultPreparationFailureKeepsTerminalOutputIdentity(t *testing.T) {
	e := &Engine{media: &Service{cipher: resultCipher{}}}
	policy := airun.Policy{UserInflightLimit: 1, UserRatePerSecond: 1, UserRateBurst: 1, MaxQueuedSeconds: 1}
	secret := "https://blocked.example/secret?token=private"
	now := time.Time{}
	safe, files := e.prepareImageResultFiles(airun.Model{ID: 1}, 2, []providerrequest.ImageItem{{URL: secret}}, []string{"media.example"}, policy, now)
	if len(safe) != 1 || len(files) != 1 || files[0].TransferStatus != TransferFailed || files[0].SourceRecoverable != 0 || files[0].ErrorCode == nil || *files[0].ErrorCode != "INVALID_SOURCE" {
		t.Fatalf("files=%+v", files)
	}
	raw, _ := json.Marshal(struct {
		Items []airun.MediaResultItem
		Files []airun.MediaResultFile
	}{safe, files})
	if strings.Contains(string(raw), secret) || strings.Contains(string(raw), "token=private") {
		t.Fatalf("plaintext source=%s", raw)
	}
}
