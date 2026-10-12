package uploadrule

import (
	"admin/server/internal/module/auth/login"
	"context"
	"strings"
	"time"
)

type FileRegistration struct {
	CosConfigID, ConfigVersion, UploadRuleID                     int64
	RuleCode, ObjectKey, UploadObjectKey, OriginalName, MimeType string
	SizeBytes                                                    int64
	ExpiresAt                                                    time.Time
}
type FileRegistrar interface {
	RegisterUpload(context.Context, auth.Identity, FileRegistration) (int64, error)
	AuthorizeObject(context.Context, auth.Identity, string) error
}
type FileUploadOption struct {
	Value            string `json:"value"`
	Label            string `json:"label"`
	Accept           string `json:"accept"`
	MaxFileSizeBytes int64  `json:"maxFileSizeBytes"`
}

func (s *Service) SetFileRegistrar(registrar FileRegistrar) { s.fileRegistrar = registrar }
func ValidateFileForTarget(in FileInput, target UploadTarget) error {
	_, e := validateCredentialFile(in, target)
	return e
}
func (s *Service) FindFileUploadTarget(ctx context.Context, platformID int64, code string) (UploadTarget, error) {
	return s.repository.FindUploadTarget(ctx, platformID, code)
}
func (s *Service) FileUploadOptions(ctx context.Context, platformID int64, purpose string) ([]FileUploadOption, error) {
	var rows []struct {
		Code, Name                          string
		MaxFileSizeBytes                    int64
		AllowedExtensions, AllowedMimeTypes StringArray
	}
	query := s.repository.db.WithContext(ctx).Table("storage_upload_rule r").Select("k.code,r.name,r.max_file_size_bytes,r.allowed_extensions,r.allowed_mime_types").Joins("JOIN storage_upload_rule_code k ON k.rule_id=r.id AND k.deleted_at IS NULL").Joins("JOIN storage_cos_config c ON c.id=r.cos_config_id AND c.is_enabled=1 AND c.deleted_at IS NULL").Where("r.platform_id=? AND r.is_enabled=1 AND r.deleted_at IS NULL", platformID)
	if purpose == "avatar" {
		query = query.Where("k.code='avatar'")
	}
	if e := query.Order("k.code").Scan(&rows).Error; e != nil {
		return nil, e
	}
	out := make([]FileUploadOption, 0, len(rows))
	for _, row := range rows {
		accept := []string{}
		for _, ext := range row.AllowedExtensions {
			accept = append(accept, "."+ext)
		}
		accept = append(accept, row.AllowedMimeTypes...)
		out = append(out, FileUploadOption{row.Code, row.Name, strings.Join(accept, ","), row.MaxFileSizeBytes})
	}
	return out, nil
}
