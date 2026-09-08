package account_test

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"admin/server/internal/module/user/account"
)

func TestSetPasswordHashConcurrentFirstWriteWins(t *testing.T) {
	db, ctx, _ := openUserDatabase(t)
	repository := account.NewRepository(db)
	created, err := repository.CreateVerifiedIdentity(ctx, account.VerifiedIdentityInput{
		IdentityKind: "email", Account: "first-password@example.com", Username: "first-password",
	})
	if err != nil {
		t.Fatal(err)
	}
	const writers = 8
	type result struct {
		hash string
		err  error
	}
	results := make(chan result, writers)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for i := 0; i < writers; i++ {
		workers.Add(1)
		go func(i int) {
			defer workers.Done()
			<-start
			hash := fmt.Sprintf("password-hash-%d", i)
			results <- result{hash, repository.SetPasswordHash(ctx, created.ID, hash, time.Now().UTC())}
		}(i)
	}
	close(start)
	workers.Wait()
	close(results)
	winners, winningHash := 0, ""
	for result := range results {
		if result.err == nil {
			winners++
			winningHash = result.hash
		}
		if result.err != nil && !errors.Is(result.err, account.ErrPasswordAlreadySet) {
			t.Errorf("unexpected loser error: %v", result.err)
		}
	}
	if winners != 1 {
		t.Fatalf("successful first-password writes = %d, want 1", winners)
	}
	credential, err := repository.FindCredentialByID(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if credential.PasswordHash != winningHash {
		t.Fatal("winning password was overwritten")
	}
}

func TestSetPasswordHashRejectsEmptyHash(t *testing.T) {
	db, ctx, _ := openUserDatabase(t)
	if err := account.NewRepository(db).SetPasswordHash(ctx, 1, "", time.Now().UTC()); !errors.Is(err, account.ErrUserDataInvalid) {
		t.Fatalf("empty hash error = %v", err)
	}
}
