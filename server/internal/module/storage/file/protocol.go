package file

import (
	"admin/server/internal/module/auth/login"
	"admin/server/internal/module/storage/uploadRule"
	"context"
	"time"
)

type ListQuery struct {
	Page, PageSize             int
	Keyword, Category, Purpose string
	MaxSize                    int64
	MIMEs, Extensions          []string
}
type FileItem struct {
	ID        int64     `json:"id"`
	FileName  string    `json:"fileName"`
	MimeType  string    `json:"mimeType"`
	SizeBytes int64     `json:"sizeBytes"`
	Category  string    `json:"category"`
	CreatedAt time.Time `json:"createdAt"`
	ObjectKey string    `json:"objectKey"`
}
type CompleteResult struct {
	FileID    int64   `json:"fileId"`
	ObjectKey string  `json:"objectKey"`
	PublicURL *string `json:"publicUrl,omitempty"`
}
type URLResult struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expiresAt"`
}
type Option struct{ Value, Label string }
type AvatarCopy struct {
	ID             int64
	ObjectKey      string
	AlreadyApplied bool
}
type UploadRules interface {
	FindFileUploadTarget(context.Context, int64, string) (uploadrule.UploadTarget, error)
	FileUploadOptions(context.Context, int64, string) ([]uploadrule.FileUploadOption, error)
}
type Registrar interface {
	RegisterUpload(context.Context, auth.Identity, uploadrule.FileRegistration) (int64, error)
}
