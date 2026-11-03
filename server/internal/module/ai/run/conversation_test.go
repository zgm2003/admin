package run

import (
	conversation "admin/server/internal/module/ai/conversation"
	"errors"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestRunConversationOwnershipAndAtomicMessageSequence(t *testing.T) {
	repo, db, ctx := runDB(t)
	conversations := conversation.NewRepository(db)
	c, e := conversations.Create(ctx, 1, 1, 1, "test", time.Now())
	if e != nil {
		t.Fatal(e)
	}
	repo.SetConversations(conversations)
	input := fixtureInput()
	input.ConversationID = &c.ID
	input.UserID = 2
	if _, _, e := repo.Create(ctx, input, fixtureSnapshot(), []byte(`{"userInflightLimit":2,"maxTextInputBytes":131072}`), time.Now()); !errors.Is(e, gorm.ErrRecordNotFound) {
		t.Fatalf("foreign conversation accepted=%v", e)
	}
	input.UserID = 1
	row, _, e := repo.Create(ctx, input, fixtureSnapshot(), []byte(`{"userInflightLimit":2,"maxTextInputBytes":131072}`), time.Now())
	if e != nil {
		t.Fatal(e)
	}
	messages, e := conversations.Messages(ctx, c.ID, 1, 1, 0, 100)
	if e != nil || len(messages) != 1 || messages[0].Sequence != 1 || messages[0].RunID == nil || *messages[0].RunID != row.ID {
		t.Fatalf("messages=%+v err=%v", messages, e)
	}
	if _, e := conversations.Messages(ctx, c.ID, 2, 1, 0, 100); !errors.Is(e, gorm.ErrRecordNotFound) {
		t.Fatalf("foreign message read=%v", e)
	}
	if _, _, e := repo.Create(ctx, input, fixtureSnapshot(), []byte(`{"userInflightLimit":2,"maxTextInputBytes":131072}`), time.Now()); e != nil {
		t.Fatal(e)
	}
	messages, e = conversations.Messages(ctx, c.ID, 1, 1, 0, 100)
	if e != nil || len(messages) != 1 {
		t.Fatalf("replay appended user message: %d %v", len(messages), e)
	}
}
