package order

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"admin/server/internal/module/payment"
	"admin/server/internal/shared/money"
	"github.com/google/uuid"
)

type CreateInput struct {
	RequestID   string
	PackageID   int64
	AmountCents int64
	PayMethod   payment.PayMethod
	ReturnURL   string
}

type createRequest struct {
	RequestID *string            `json:"requestId"`
	PackageID *int64             `json:"packageId"`
	Amount    *string            `json:"amount"`
	PayMethod *payment.PayMethod `json:"payMethod"`
	ReturnURL *string            `json:"returnUrl"`
}

func (r createRequest) input() (CreateInput, error) {
	if r.RequestID == nil || r.PayMethod == nil || r.ReturnURL == nil || (r.PackageID == nil) == (r.Amount == nil) {
		return CreateInput{}, fmt.Errorf("requestId, payMethod, returnUrl and exactly one recharge source are required")
	}
	input := CreateInput{RequestID: strings.TrimSpace(*r.RequestID), PayMethod: *r.PayMethod, ReturnURL: *r.ReturnURL}
	if r.PackageID != nil {
		if *r.PackageID < 1 {
			return CreateInput{}, fmt.Errorf("packageId invalid")
		}
		input.PackageID = *r.PackageID
	} else {
		cents, err := money.ParseRMBCents(*r.Amount)
		if err != nil || cents < 1 {
			return CreateInput{}, fmt.Errorf("amount invalid")
		}
		input.AmountCents = cents
	}
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
	if input.PackageID < 0 || input.AmountCents < 0 || (input.PackageID == 0) == (input.AmountCents == 0) || (input.PayMethod != payment.PayMethodWeb && input.PayMethod != payment.PayMethodH5) {
		return fmt.Errorf("recharge source or payMethod invalid")
	}
	if _, err := money.CentsToUnits(input.AmountCents); err != nil {
		return fmt.Errorf("amount exceeds wallet limit")
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
	facts := input.RequestID + "\x00" + fmt.Sprintf("%d", input.PackageID) + "\x00" + fmt.Sprintf("%d", input.PayMethod) + "\x00" + input.ReturnURL
	// Preserve existing package fingerprints so pre-migration requests still replay.
	if input.PackageID == 0 {
		facts += "\x00amount=" + strconv.FormatInt(input.AmountCents, 10)
	}
	digest := sha256.Sum256([]byte(facts))
	return digest[:]
}

func fingerprintText(value []byte) string { return hex.EncodeToString(value) }
