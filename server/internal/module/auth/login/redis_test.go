package auth

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"admin/server/internal/config"
	projectredis "admin/server/internal/redis"
	"github.com/joho/godotenv"
)

func openAuthRedis(t *testing.T) *projectredis.Client {
	t.Helper()
	if testing.Short() {
		t.Skip("Redis integration test")
	}
	if err := godotenv.Load("../../../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	client, err := projectredis.Open(context.Background(), settings.RedisURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func newVerificationStoreForTest(t *testing.T) VerificationCodeStore {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return NewVerificationCodeStore(openAuthRedis(t), key)
}

func TestVerificationProofDigestBindsChallengeID(t *testing.T) {
	store := &verificationCodeStore{hmacKey: []byte("01234567890123456789012345678901")}
	first := store.ProofDigest("challenge-a", "123456")
	second := store.ProofDigest("challenge-b", "123456")
	if first == "" || second == "" || first == second {
		t.Fatalf("proof digests do not bind challengeId: first=%q second=%q", first, second)
	}
}

func TestVerificationCodeWrongChallengeDoesNotConsumeCorrectProof(t *testing.T) {
	store := newVerificationStoreForTest(t)
	key := fmt.Sprintf("auth:verify-code:v2:admin:login:email:challenge-%d", time.Now().UnixNano())
	ctx := context.Background()

	if acquired, err := store.AcquireDelivery(ctx, key, "lease-a", 10*time.Second); err != nil || !acquired {
		t.Fatalf("AcquireDelivery = %v, %v", acquired, err)
	}
	correctDigest := store.ProofDigest("challenge-a", "123456")
	if err := store.Put(ctx, key, correctDigest, "lease-a", time.Minute); err != nil {
		t.Fatal(err)
	}
	wrongDigest := store.ProofDigest("challenge-b", "123456")
	if consumed, err := store.Consume(ctx, key, wrongDigest); err != nil || consumed {
		t.Fatalf("wrong challenge consume = %v, %v", consumed, err)
	}
	if valid, err := store.Check(ctx, key, correctDigest); err != nil || !valid {
		t.Fatalf("correct proof after wrong challenge = %v, %v", valid, err)
	}
	if consumed, err := store.Consume(ctx, key, correctDigest); err != nil || !consumed {
		t.Fatalf("correct challenge consume = %v, %v", consumed, err)
	}
}

func TestVerificationCodeConsumeIsSingleUse(t *testing.T) {
	store := newVerificationStoreForTest(t)
	key := fmt.Sprintf("auth:verify-code:v1:admin:login:email:test-%d", time.Now().UnixNano())
	ctx := context.Background()

	if acquired, err := store.AcquireDelivery(ctx, key, "lease-a", 10*time.Second); err != nil || !acquired {
		t.Fatalf("AcquireDelivery = %v, %v", acquired, err)
	}
	if err := store.Put(ctx, key, "digest-a", "lease-a", time.Minute); err != nil {
		t.Fatal(err)
	}
	first, err := store.Consume(ctx, key, "digest-a")
	if err != nil || !first {
		t.Fatalf("first consume = %v, %v", first, err)
	}
	second, err := store.Consume(ctx, key, "digest-a")
	if err != nil || second {
		t.Fatalf("replay consume = %v, %v", second, err)
	}
}

func TestVerificationCodeWrongCodeDoesNotDeleteCorrectCode(t *testing.T) {
	store := newVerificationStoreForTest(t)
	key := fmt.Sprintf("auth:verify-code:v1:admin:login:email:wrong-%d", time.Now().UnixNano())
	ctx := context.Background()

	if _, err := store.AcquireDelivery(ctx, key, "lease-a", 10*time.Second); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(ctx, key, "digest-correct", "lease-a", time.Minute); err != nil {
		t.Fatal(err)
	}
	if consumed, err := store.Consume(ctx, key, "digest-wrong"); err != nil || consumed {
		t.Fatalf("wrong consume = %v, %v", consumed, err)
	}
	if ok, err := store.Check(ctx, key, "digest-correct"); err != nil || !ok {
		t.Fatalf("correct code after wrong consume = %v, %v", ok, err)
	}
	if consumed, err := store.Consume(ctx, key, "digest-correct"); err != nil || !consumed {
		t.Fatalf("correct consume = %v, %v", consumed, err)
	}
}

func TestVerificationCodeConsumeManyIsAllOrNothingAndSingleUse(t *testing.T) {
	client := openAuthRedis(t)
	keyMaterial := []byte("01234567890123456789012345678901")
	store := NewVerificationCodeStore(client, keyMaterial)
	ctx := context.Background()
	keys := []string{
		store.VerificationKey("admin", "bind_email", "email", "current@example.com"),
		store.VerificationKey("admin", "bind_email", "email", "next@example.com"),
	}
	leaseKeys := []string{keys[0] + verificationCodeLeaseSuffix, keys[1] + verificationCodeLeaseSuffix}
	t.Cleanup(func() { _ = client.DeleteMany(context.Background(), append(keys, leaseKeys...)) })
	_ = client.DeleteMany(ctx, append(keys, leaseKeys...))
	digests := []string{store.ProofDigest("current-challenge", "111111"), store.ProofDigest("next-challenge", "222222")}
	for index := range keys {
		lease := fmt.Sprintf("lease-%d", index)
		acquired, err := store.AcquireDelivery(ctx, keys[index], lease, time.Minute)
		if err != nil || !acquired {
			t.Fatalf("acquire key %d = %v,%v", index, acquired, err)
		}
		if err := store.Put(ctx, keys[index], digests[index], lease, time.Minute); err != nil {
			t.Fatalf("put key %d: %v", index, err)
		}
	}

	wrong := append([]string(nil), digests...)
	wrong[1] = store.ProofDigest("next-challenge", "999999")
	if consumed, err := store.ConsumeMany(ctx, keys, wrong); err != nil || consumed {
		t.Fatalf("partial mismatch consumed=%v err=%v", consumed, err)
	}
	if consumed, err := store.ConsumeMany(ctx, keys, digests); err != nil || !consumed {
		t.Fatalf("correct proofs after mismatch consumed=%v err=%v", consumed, err)
	}
	if consumed, err := store.ConsumeMany(ctx, keys, digests); err != nil || consumed {
		t.Fatalf("reused proofs consumed=%v err=%v", consumed, err)
	}
}

func TestVerificationCodeDeleteIfOwnedRequiresToken(t *testing.T) {
	store := newVerificationStoreForTest(t)
	key := fmt.Sprintf("auth:verify-code:v1:admin:login:email:owned-%d", time.Now().UnixNano())
	ctx := context.Background()

	if _, err := store.AcquireDelivery(ctx, key, "lease-a", 10*time.Second); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(ctx, key, "digest-a", "lease-a", time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteIfOwned(ctx, key, "wrong-token"); err == nil {
		t.Fatal("DeleteIfOwned accepted a wrong lease token")
	}
	if ok, err := store.Check(ctx, key, "digest-a"); err != nil || !ok {
		t.Fatalf("code deleted by wrong token: ok=%v err=%v", ok, err)
	}
	if err := store.DeleteIfOwned(ctx, key, "lease-a"); err != nil {
		t.Fatalf("DeleteIfOwned with correct token: %v", err)
	}
	if ok, err := store.Check(ctx, key, "digest-a"); err != nil || ok {
		t.Fatalf("code still present after owned delete: ok=%v err=%v", ok, err)
	}
}

func TestVerificationCodePutReplacesPriorCodeUnderCurrentLease(t *testing.T) {
	store := newVerificationStoreForTest(t)
	key := fmt.Sprintf("auth:verify-code:v2:admin:login:email:replace-%d", time.Now().UnixNano())
	ctx := context.Background()

	if acquired, err := store.AcquireDelivery(ctx, key, "lease-a", 10*time.Second); err != nil || !acquired {
		t.Fatalf("first AcquireDelivery = %v, %v", acquired, err)
	}
	if err := store.Put(ctx, key, "digest-a", "lease-a", time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := store.ReleaseDelivery(ctx, key, "lease-a"); err != nil {
		t.Fatal(err)
	}
	if acquired, err := store.AcquireDelivery(ctx, key, "lease-b", 10*time.Second); err != nil || !acquired {
		t.Fatalf("second AcquireDelivery = %v, %v", acquired, err)
	}
	if err := store.Put(ctx, key, "digest-b", "lease-b", time.Minute); err != nil {
		t.Fatalf("second delivery did not replace the prior code: %v", err)
	}
	if valid, err := store.Check(ctx, key, "digest-b"); err != nil || !valid {
		t.Fatalf("new code after replace = %v, %v", valid, err)
	}
	if valid, err := store.Check(ctx, key, "digest-a"); err != nil || valid {
		t.Fatalf("old code still valid after replace = %v, %v", valid, err)
	}
}

func TestVerificationCodePutRejectsMissingOrMismatchedLease(t *testing.T) {
	store := newVerificationStoreForTest(t)
	key := fmt.Sprintf("auth:verify-code:v2:admin:login:email:lease-%d", time.Now().UnixNano())
	ctx := context.Background()
	if err := store.Put(ctx, key, "digest-a", "lease-a", time.Minute); err == nil {
		t.Fatal("Put accepted a missing lease")
	}
	if acquired, err := store.AcquireDelivery(ctx, key, "lease-a", 10*time.Second); err != nil || !acquired {
		t.Fatalf("AcquireDelivery = %v, %v", acquired, err)
	}
	if err := store.Put(ctx, key, "digest-a", "lease-wrong", time.Minute); err == nil {
		t.Fatal("Put accepted a mismatched lease token")
	}
}

func TestVerificationCodePutRejectsOutOfRangeTTL(t *testing.T) {
	store := newVerificationStoreForTest(t)
	ctx := context.Background()
	for _, test := range []struct {
		name string
		ttl  time.Duration
		ok   bool
	}{
		{"below minimum", 30 * time.Second, false},
		{"one minute", time.Minute, true},
		{"five minutes", 5 * time.Minute, true},
		{"sixty minutes", 60 * time.Minute, true},
		{"over maximum", 61 * time.Minute, false},
	} {
		key := fmt.Sprintf("auth:verify-code:v2:admin:login:email:ttl-%d", time.Now().UnixNano())
		if _, err := store.AcquireDelivery(ctx, key, "lease", 10*time.Second); err != nil {
			t.Fatal(err)
		}
		err := store.Put(ctx, key, "digest", "lease", test.ttl)
		if test.ok && err != nil {
			t.Fatalf("%s: Put(%v) = %v", test.name, test.ttl, err)
		}
		if !test.ok && err == nil {
			t.Fatalf("%s: Put accepted TTL %v", test.name, test.ttl)
		}
	}
}

func TestVerificationCodeRejectsCorruptValueFields(t *testing.T) {
	store := newVerificationStoreForTest(t)
	client := openAuthRedis(t)
	ctx := context.Background()
	key := fmt.Sprintf("auth:verify-code:v2:admin:login:email:corrupt-%d", time.Now().UnixNano())
	t.Cleanup(func() { _ = client.Delete(ctx, key) })

	for _, raw := range []string{
		`{"digest":"digest-a"}`,
		`{"digest":"digest-a","leaseToken":"lease-a","email":"pii@example.com"}`,
		`{"digest":"digest-a","leaseToken":"lease-a"} trailing`,
	} {
		if err := client.SetString(ctx, key, raw, time.Minute); err != nil {
			t.Fatal(err)
		}
		if valid, err := store.Check(ctx, key, "digest-a"); err == nil || valid {
			t.Fatalf("Check accepted corrupt value %q: valid=%v err=%v", raw, valid, err)
		}
		if consumed, err := store.Consume(ctx, key, "digest-a"); err == nil || consumed {
			t.Fatalf("Consume accepted corrupt value %q: consumed=%v err=%v", raw, consumed, err)
		}
	}
}

func TestVerificationCodeCheckAttemptLimitsAccountAndIPWithoutDeletingCode(t *testing.T) {
	store := newVerificationStoreForTest(t)
	ctx := context.Background()
	runID := uint64(time.Now().UnixNano())
	accountIP := fmt.Sprintf("2001:db8:%x:%x::1", uint16(runID>>48), uint16(runID>>32))
	sharedIP := fmt.Sprintf("2001:db8:%x:%x::2", uint16(runID>>16), uint16(runID))
	key := fmt.Sprintf("auth:verify-code:v1:admin:login:email:attempt-%d", runID)
	if acquired, err := store.AcquireDelivery(ctx, key, "lease-a", 10*time.Second); err != nil || !acquired {
		t.Fatalf("AcquireDelivery = %v, %v", acquired, err)
	}
	if err := store.Put(ctx, key, "digest-correct", "lease-a", time.Minute); err != nil {
		t.Fatal(err)
	}

	for attempt := 0; attempt < 10; attempt++ {
		valid, limited, err := store.CheckAttempt(ctx, key, "digest-wrong", accountIP)
		if err != nil || valid || limited {
			t.Fatalf("account attempt %d = valid:%v limited:%v err:%v", attempt+1, valid, limited, err)
		}
	}
	if valid, limited, err := store.CheckAttempt(ctx, key, "digest-correct", accountIP); err != nil || valid || !limited {
		t.Fatalf("limited correct attempt = valid:%v limited:%v err:%v", valid, limited, err)
	}
	if valid, err := store.Check(ctx, key, "digest-correct"); err != nil || !valid {
		t.Fatalf("correct code was deleted after limited attempts = %v, %v", valid, err)
	}

	for attempt := 0; attempt < 30; attempt++ {
		otherKey := fmt.Sprintf("auth:verify-code:v1:admin:login:email:ip-%d-%d", time.Now().UnixNano(), attempt)
		valid, limited, err := store.CheckAttempt(ctx, otherKey, "digest-wrong", sharedIP)
		if err != nil || valid || limited {
			t.Fatalf("IP attempt %d = valid:%v limited:%v err:%v", attempt+1, valid, limited, err)
		}
	}
	otherKey := fmt.Sprintf("auth:verify-code:v1:admin:login:email:ip-limited-%d", time.Now().UnixNano())
	if valid, limited, err := store.CheckAttempt(ctx, otherKey, "digest-wrong", sharedIP); err != nil || valid || !limited {
		t.Fatalf("limited IP attempt = valid:%v limited:%v err:%v", valid, limited, err)
	}
}
