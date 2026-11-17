package chat

import "testing"

func TestDeleteMessagesRequestDeduplicatesIDs(t *testing.T) {
	ids, err := (deleteMessagesRequest{IDs: []int64{3, 1, 3}}).ids()
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != 3 || ids[1] != 1 {
		t.Fatalf("ids = %v", ids)
	}
}

func TestDeleteMessagesRequestRejectsInvalidIDs(t *testing.T) {
	cases := [][]int64{
		nil,
		{0},
		make([]int64, maxDeleteMessages+1),
	}
	for _, ids := range cases {
		if _, err := (deleteMessagesRequest{IDs: ids}).ids(); err == nil {
			t.Fatalf("ids %v were accepted", ids)
		}
	}
}
