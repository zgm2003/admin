package order

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"admin/server/internal/module/payment"
	"github.com/google/uuid"
)

type CreateInput struct {
	RequestID string
	PackageID int64
	PayMethod payment.PayMethod
	ReturnURL string
}

type createRequest struct {
	RequestID *string            `json:"requestId"`
	PackageID *int64             `json:"packageId"`
	PayMethod *payment.PayMethod `json:"payMethod"`
	ReturnURL *string            `json:"returnUrl"`
}

func (r createRequest) input() (CreateInput, error) {
	if r.RequestID == nil || r.PackageID == nil || r.PayMethod == nil || r.ReturnURL == nil {
		return CreateInput{}, fmt.Errorf("requestId, packageId, payMethod and returnUrl are required")
	}
	input := CreateInput{RequestID: strings.TrimSpace(*r.RequestID), PackageID: *r.PackageID, PayMethod: *r.PayMethod, ReturnURL: *r.ReturnURL}
	if err := validateCreateInput(input); err != nil {
		return CreateInput{}, err
	}
	return input, nil
}

func validateCreateInput(input CreateInput) error {
	parsed, err := uuid.Parse(input.RequestID)
	if err != nil || parsed == uuid.Nil || len(input.RequestID) != 36 || parsed.String() != input.RequestID || strings.TrimSpace(input.RequestID) != input.RequestID {
		return fmt.Errorf("requestId invalid")
	}
	if input.PackageID < 1 || (input.PayMethod != payment.PayMethodWeb && input.PayMethod != payment.PayMethodH5) {
		return fmt.Errorf("packageId or payMethod invalid")
	}
	if _, err := parseReturnURL(input.ReturnURL); err != nil {
		return err
	}
	return nil
}

func parseReturnURL(raw string) (*url.URL, error) {
	if len(raw) == 0 || len(raw) > 2048 || strings.TrimSpace(raw) != raw {
		return nil, fmt.Errorf("returnUrl invalid")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil || strings.Contains(raw, "#") || parsed.Opaque != "" || parsed.Scheme == "" || parsed.Hostname() == "" || strings.ContainsAny(parsed.Host, "\\/\t\r\n @*") || strings.HasSuffix(parsed.Host, ":") || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return nil, fmt.Errorf("returnUrl invalid")
	}
	if port := parsed.Port(); port != "" {
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			return nil, fmt.Errorf("returnUrl invalid")
		}
	}
	return parsed, nil
}

func requestFingerprint(input CreateInput) []byte {
	digest := sha256.Sum256([]byte(input.RequestID + "\x00" + fmt.Sprintf("%d", input.PackageID) + "\x00" + fmt.Sprintf("%d", input.PayMethod) + "\x00" + input.ReturnURL))
	return digest[:]
}

func fingerprintText(value []byte) string { return hex.EncodeToString(value) }
