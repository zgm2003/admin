package callbackevent

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"admin/server/internal/module/payment"
	"admin/server/internal/module/payment/order"
	"admin/server/internal/payment/alipay"
	"gorm.io/gorm"
)

type eventRepositoryStub struct {
	rows                          map[int64]Model
	orderID                       int64
	findErr, createErr, updateErr error
	updates, creates              int
	beforeUpdate                  func(*eventRepositoryStub, int64)
}

func (r *eventRepositoryStub) FindByDedupe(_ context.Context, configID, version int64, key []byte) (Model, error) {
	if r.findErr != nil {
		return Model{}, r.findErr
	}
	for _, row := range r.rows {
		if row.ConfigID == configID && row.ConfigVersion == version && string(row.DedupeKey) == string(key) {
			return row, nil
		}
	}
	return Model{}, gorm.ErrRecordNotFound
}
func (r *eventRepositoryStub) Create(_ context.Context, row *Model) error {
	r.creates++
	if r.createErr != nil {
		return r.createErr
	}
	if r.rows == nil {
		r.rows = make(map[int64]Model)
	}
	row.ID = int64(len(r.rows) + 1)
	r.rows[row.ID] = *row
	return nil
}
func (r *eventRepositoryStub) UpdateFailure(ctx context.Context, id int64, status ProcessStatus, code ErrorCode, now time.Time) error {
	return r.UpdateResult(ctx, id, status, &code, now)
}
func (r *eventRepositoryStub) UpdateResult(_ context.Context, id int64, status ProcessStatus, code *ErrorCode, now time.Time) error {
	r.updates++
	if r.beforeUpdate != nil {
		r.beforeUpdate(r, id)
	}
	if r.updateErr != nil {
		return r.updateErr
	}
	row, ok := r.rows[id]
	if !ok || row.ProcessStatus != ProcessPending {
		return gorm.ErrRecordNotFound
	}
	row.ProcessStatus, row.ErrorCode, row.UpdatedAt = status, code, now
	r.rows[id] = row
	return nil
}
func (r *eventRepositoryStub) Find(_ context.Context, id int64) (Model, error) {
	row, ok := r.rows[id]
	if !ok {
		return Model{}, gorm.ErrRecordNotFound
	}
	return row, nil
}
func (r *eventRepositoryStub) FindOrderID(context.Context, string) (*int64, error) {
	if r.orderID == 0 {
		return nil, nil
	}
	id := r.orderID
	return &id, nil
}
func (r *eventRepositoryStub) CountByOrder(context.Context, int64) (int64, error) { return 0, nil }
func (r *eventRepositoryStub) ListByOrder(context.Context, int64, int, int) ([]Model, error) {
	return []Model{}, nil
}

type eventRuntimeStub struct {
	credentials alipay.Credentials
	err         error
}

func (r eventRuntimeStub) Credentials(context.Context, int64, int64) (alipay.Credentials, error) {
	return r.credentials, r.err
}

type eventConfirmerStub struct {
	repository *eventRepositoryStub
	err        error
	calls      int
	last       order.ConfirmedPayment
}

func (c *eventConfirmerStub) Confirm(_ context.Context, evidence order.ConfirmedPayment, id *int64) (order.SettlementResult, error) {
	c.calls++
	c.last = evidence
	if c.err != nil {
		return order.SettlementResult{}, c.err
	}
	// Simulate only the committed repository boundary, not a wallet transaction.
	row := c.repository.rows[*id]
	row.ProcessStatus, row.ErrorCode = ProcessSuccess, nil
	c.repository.rows[*id] = row
	return order.SettlementResult{OrderID: 11, TransactionID: 1}, nil
}

type eventFixture struct {
	service     *Service
	repo        *eventRepositoryStub
	confirmer   *eventConfirmerStub
	credentials alipay.Credentials
	key         *rsa.PrivateKey
}

func newEventFixture(t *testing.T) eventFixture {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Callback RSA2 Fixture"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	credentials := alipay.Credentials{AppID: "20260001", SellerID: "20880001", Environment: payment.EnvironmentSandbox, AlipayCertPEM: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))}
	repo := &eventRepositoryStub{orderID: 11}
	confirmer := &eventConfirmerStub{repository: repo}
	service := NewService(repo)
	service.SetRuntime(eventRuntimeStub{credentials: credentials})
	service.SetConfirmer(confirmer)
	service.now = func() time.Time { return now }
	return eventFixture{service, repo, confirmer, credentials, key}
}
func (f eventFixture) signed(t *testing.T, change func(url.Values)) url.Values {
	t.Helper()
	values := url.Values{"notify_id": {"NOTIFY-1"}, "out_trade_no": {"ORDER-1"}, "trade_no": {"TRADE-1"}, "app_id": {f.credentials.AppID}, "seller_id": {f.credentials.SellerID}, "trade_status": {"TRADE_SUCCESS"}, "total_amount": {"3.00"}, "gmt_payment": {"2026-10-16 08:00:00"}}
	if change != nil {
		change(values)
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		if values.Get(key) != "" {
			parts = append(parts, key+"="+values.Get(key))
		}
	}
	digest := sha256.Sum256([]byte(strings.Join(parts, "&")))
	signature, err := rsa.SignPKCS1v15(rand.Reader, f.key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	values.Set("sign_type", "RSA2")
	values.Set("sign", base64.StdEncoding.EncodeToString(signature))
	return values
}

func TestReceivePersistsOnlyClosedSignedFactsAndReplaysOnce(t *testing.T) {
	f := newEventFixture(t)
	values := f.signed(t, func(v url.Values) {
		v.Set("buyer_logon_id", "PRIVATE_BUYER")
		v.Set("passback_params", "TOKEN=PRIVATE_TOKEN")
	})
	for i := 0; i < 2; i++ {
		result, err := f.service.Receive(context.Background(), 3, 4, values)
		if err != nil || result.EventID != 1 || result.Replayed != (i == 1) {
			t.Fatalf("delivery %d: result=%+v err=%v", i, result, err)
		}
	}
	if f.repo.creates != 1 || f.confirmer.calls != 1 {
		t.Fatalf("creates=%d confirms=%d", f.repo.creates, f.confirmer.calls)
	}
	row := f.repo.rows[1]
	if row.OrderID == nil || *row.OrderID != 11 {
		t.Fatalf("matched order not recorded at receive: %+v", row.OrderID)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(row.Payload, &fields); err != nil || len(fields) != 8 {
		t.Fatalf("payload is not closed: %s %v", row.Payload, err)
	}
	for _, private := range []string{"PRIVATE_BUYER", "PRIVATE_TOKEN", "sign_type", "passback_params"} {
		if strings.Contains(string(row.Payload), private) {
			t.Fatal("private/raw request escaped into callback evidence")
		}
	}
}

func TestReceiveSameNotifyIDRejectsChangedFactsWithoutCreatingNewEvidence(t *testing.T) {
	f := newEventFixture(t)
	if _, err := f.service.Receive(context.Background(), 3, 4, f.signed(t, nil)); err != nil {
		t.Fatal(err)
	}
	original := string(f.repo.rows[1].Payload)
	_, err := f.service.Receive(context.Background(), 3, 4, f.signed(t, func(v url.Values) { v.Set("total_amount", "4.00") }))
	if !errors.Is(err, ErrEventFactConflict) {
		t.Fatalf("changed facts error=%v, want fact conflict", err)
	}
	if f.repo.creates != 1 || f.confirmer.calls != 1 || string(f.repo.rows[1].Payload) != original {
		t.Fatal("conflict created or altered settled evidence")
	}
}

func TestReceiveRejectedEvidenceNeverAcknowledgesReplay(t *testing.T) {
	f := newEventFixture(t)
	f.confirmer.err = order.ErrOrderMismatch
	for i := 0; i < 2; i++ {
		if _, err := f.service.Receive(context.Background(), 3, 4, f.signed(t, nil)); err == nil {
			t.Fatalf("rejected delivery %d acknowledged success", i)
		}
	}
	row := f.repo.rows[1]
	if row.ProcessStatus != ProcessRejected || row.ErrorCode == nil || *row.ErrorCode != ErrorOrderMismatch || row.OrderID == nil {
		t.Fatalf("rejection diagnostic lost: %+v", row)
	}
	if f.confirmer.calls != 1 {
		t.Fatalf("rejected replay confirmed %d times", f.confirmer.calls)
	}
}

func TestReceiveDistinguishesNonPaymentAndUnknownProviderStatuses(t *testing.T) {
	for _, status := range []string{"WAIT_BUYER_PAY", "TRADE_CLOSED", "TRADE_UNKNOWN"} {
		t.Run(status, func(t *testing.T) {
			f := newEventFixture(t)
			values := f.signed(t, func(v url.Values) { v.Set("trade_status", status); v.Del("gmt_payment") })
			for i := 0; i < 2; i++ {
				result, err := f.service.Receive(context.Background(), 3, 4, values)
				if status == "TRADE_UNKNOWN" {
					if err == nil {
						t.Fatal("unknown state acknowledged")
					}
				} else if err != nil || !result.Ignored {
					t.Fatalf("known nonpayment result=%+v err=%v", result, err)
				}
			}
			row := f.repo.rows[1]
			if status == "TRADE_UNKNOWN" {
				if row.ProcessStatus != ProcessPending || row.ErrorCode == nil || *row.ErrorCode != ErrorUnknownStatus {
					t.Fatalf("unknown state not retained pending: %+v", row)
				}
			} else if row.ProcessStatus != ProcessIgnored || row.ErrorCode != nil {
				t.Fatalf("normal ignored status has spurious error: %+v", row)
			}
			if f.confirmer.calls != 0 {
				t.Fatal("nonpayment state reached settlement")
			}
		})
	}
}

func TestReceiveKeepsPendingEvidenceOnDependencyFailureAndRecovers(t *testing.T) {
	f := newEventFixture(t)
	f.confirmer.err = order.ErrBalanceOverflow
	if _, err := f.service.Receive(context.Background(), 3, 4, f.signed(t, nil)); err == nil {
		t.Fatal("overflow acknowledged")
	}
	row := f.repo.rows[1]
	if row.ProcessStatus != ProcessPending || row.ErrorCode == nil || *row.ErrorCode != ErrorBalance || row.OrderID == nil {
		t.Fatalf("recoverable proof lost: %+v", row)
	}
	original := string(row.Payload)
	f.confirmer.err = nil
	if _, err := f.service.Receive(context.Background(), 3, 4, f.signed(t, nil)); err != nil {
		t.Fatal(err)
	}
	if f.repo.rows[1].ProcessStatus != ProcessSuccess || string(f.repo.rows[1].Payload) != original {
		t.Fatal("retry lost original evidence")
	}
}

func TestReceiveSignedIdentityMismatchIsAuditableAndRejected(t *testing.T) {
	f := newEventFixture(t)
	for i := 0; i < 2; i++ {
		if _, err := f.service.Receive(context.Background(), 3, 4, f.signed(t, func(v url.Values) { v.Set("seller_id", "OTHER-SELLER") })); err == nil {
			t.Fatal("wrong signed merchant accepted")
		}
	}
	row := f.repo.rows[1]
	if row.ProcessStatus != ProcessRejected || row.ErrorCode == nil || *row.ErrorCode != ErrorIdentity || row.OrderID == nil || f.confirmer.calls != 0 {
		t.Fatalf("identity mismatch not safely recorded: %+v confirms=%d", row, f.confirmer.calls)
	}
}

func TestSettlementInvariantIsNotMisclassifiedAsBalanceOverflow(t *testing.T) {
	if got := classifyError(order.ErrSettlementInvariant); got != ErrorDependency {
		t.Fatalf("invariant classification=%d", got)
	}
}

func TestReceiveRejectsInvalidCredentialEnvironmentBeforeAudit(t *testing.T) {
	for _, environment := range []payment.Environment{0, 3} {
		f := newEventFixture(t)
		f.credentials.Environment = environment
		f.service.SetRuntime(eventRuntimeStub{credentials: f.credentials})
		if _, err := f.service.Receive(context.Background(), 3, 4, f.signed(t, nil)); err == nil {
			t.Fatalf("invalid credential environment %d accepted", environment)
		}
		if f.repo.creates != 0 || f.confirmer.calls != 0 {
			t.Fatal("wrong environment persisted or credited")
		}
	}
}

func TestReceiveCarriesVerifiedEnvironmentToSettlement(t *testing.T) {
	f := newEventFixture(t)
	if _, err := f.service.Receive(context.Background(), 3, 4, f.signed(t, nil)); err != nil {
		t.Fatal(err)
	}
	if f.confirmer.last.Environment != payment.EnvironmentSandbox {
		t.Fatal("verified environment omitted from settlement proof")
	}
}

func TestConcurrentOutcomeCASReadsCanonicalCommittedFacts(t *testing.T) {
	f := newEventFixture(t)
	f.confirmer.err = order.ErrBalanceOverflow
	f.repo.beforeUpdate = func(repo *eventRepositoryStub, id int64) {
		row := repo.rows[id]
		orderID := int64(11)
		row.OrderID = &orderID
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(row.Payload, &fields); err != nil {
			t.Fatal(err)
		}
		body, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		row.Payload, row.ProcessStatus, row.ErrorCode = body, ProcessSuccess, nil
		repo.rows[id] = row
		repo.updateErr = gorm.ErrRecordNotFound
	}
	result, err := f.service.Receive(context.Background(), 3, 4, f.signed(t, nil))
	if err != nil || result.EventID != 1 || !result.Replayed {
		t.Fatalf("committed concurrent outcome result=%+v err=%v", result, err)
	}
}

func TestOutcomeCASMissDoesNotInventSuccessForPendingEvidence(t *testing.T) {
	f := newEventFixture(t)
	f.confirmer.err, f.repo.updateErr = order.ErrBalanceOverflow, gorm.ErrRecordNotFound
	if _, err := f.service.Receive(context.Background(), 3, 4, f.signed(t, nil)); err == nil {
		t.Fatal("unconfirmed CAS miss acknowledged")
	}
}
