package media

import (
	airun "admin/server/internal/module/ai/run"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// PrepareResultFile only constructs an initial fact. It performs no repository,
// scheduler, download, or storage call; the Run repository commits it atomically
// with the confirmed provider result. A source encryption failure remains an
// explicit terminal transfer fact instead of erasing known provider usage.
func (s *Service) PrepareResultFile(in InlineImport, sourceURL string, hosts []string, now time.Time) (airun.MediaResultFile, error) {
	if s == nil || in.RunID < 1 || in.ProviderID < 1 || in.Ordinal < 0 || in.RequestSeed == "" || in.TransferTimeout <= 0 || now.IsZero() {
		return airun.MediaResultFile{}, errors.New("media result input invalid")
	}
	kind := in.SourceKind
	if sourceURL != "" {
		kind = SourceURL
	}
	if kind == 0 {
		kind = SourceBase64
	}
	if kind < SourceURL || kind > SourceStream {
		return airun.MediaResultFile{}, errors.New("media result source invalid")
	}
	deadline := now.Add(in.TransferTimeout)
	key := deterministicUUID(fmt.Sprintf("%d:%d:%d:%s", in.RunID, in.ProviderID, in.Ordinal, in.RequestSeed))
	metadata, _ := json.Marshal(map[string]any{"allowedHosts": hosts, "originalName": in.OriginalName})
	result := airun.MediaResultFile{RunID: in.RunID, Role: FileRoleGenerated, Ordinal: in.Ordinal, ImportRequestKey: key, SourceKind: kind, SourceRecoverable: 0, TransferDeadline: deadline, MIMEType: &in.MIME, Metadata: metadata, TransferStatus: Transfering, TransferAttempts: 1, NextTransferAt: &deadline, CreatedAt: now, UpdatedAt: now}
	if kind == SourceURL {
		sealed, keyID, err := s.cipherSeal(sourceURL)
		if err != nil || sealed == "" || keyID == "" || sourceURL == "" {
			markResultUnrecoverable(&result, "SOURCE_UNRECOVERABLE", "媒体来源未能安全保存")
			return result, nil
		}
		result.SourceCiphertext = &sealed
		result.EncryptionKeyID = &keyID
		result.SourceRecoverable = 1
		result.TransferStatus = TransferQueued
		result.TransferAttempts = 0
		result.NextTransferAt = &now
	}
	return result, nil
}
func markResultUnrecoverable(result *airun.MediaResultFile, code, message string) {
	result.TransferStatus = TransferFailed
	result.SourceRecoverable = 0
	result.SourceCiphertext = nil
	result.EncryptionKeyID = nil
	result.NextTransferAt = nil
	result.ErrorCode = &code
	result.ErrorMessage = &message
}

// FailedResultFile preserves the output identity even when its source metadata
// cannot be prepared. An invalid budget expires immediately; it never invents
// a new transfer window or stores the unsafe source as a fallback.
func (s *Service) FailedResultFile(in InlineImport, now time.Time, code string) airun.MediaResultFile {
	deadline := now
	if in.TransferTimeout > 0 {
		deadline = now.Add(in.TransferTimeout)
	}
	kind := in.SourceKind
	if kind < SourceURL || kind > SourceStream {
		kind = SourceBase64
	}
	key := deterministicUUID(fmt.Sprintf("%d:%d:%d:%s", in.RunID, in.ProviderID, in.Ordinal, in.RequestSeed))
	result := airun.MediaResultFile{RunID: in.RunID, Role: FileRoleGenerated, Ordinal: in.Ordinal, ImportRequestKey: key, SourceKind: kind, SourceRecoverable: 0, TransferDeadline: deadline, Metadata: json.RawMessage(`{}`), TransferStatus: TransferFailed, CreatedAt: now, UpdatedAt: now}
	markResultUnrecoverable(&result, code, "媒体来源无法安全保存")
	return result
}
