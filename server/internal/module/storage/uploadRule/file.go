package uploadrule

import (
	"admin/server/internal/module/auth/login"
	"admin/server/internal/shared/apperror"
	"context"
	"errors"
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
	// 编码是调用场景的后端契约，不是让文件库用户选择的存储配置。
	code := "file"
	switch purpose {
	case "":
	case "avatar":
		code = "avatar"
	default:
		return nil, apperror.InvalidRequest(errors.New("unsupported file upload purpose"))
	}
	if platformID < 1 {
		return nil, apperror.InvalidRequest(errors.New("invalid file upload platform"))
	}
	rows, err := s.repository.FileUploadOptions(ctx, platformID, code)
	if err != nil {
		return nil, err
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
