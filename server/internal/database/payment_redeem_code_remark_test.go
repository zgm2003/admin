package database_test

import (
	"encoding/json"
	"testing"
	"time"

	redeemcode "admin/server/internal/module/payment/redeemCode"
	"gorm.io/gorm"
)

func TestPaymentRedeemBatchRemarkAcrossAdminReads(t *testing.T) {
	db, ctx := openPaymentRedeemSchema(t, "test_redeem_remark")
	applyPaymentRedeemMigration(t, db, ctx)
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(plaintextMigration(t)).Error }); err != nil {
		t.Fatal(err)
	}
	repository := redeemcode.NewRepository(db)
	service := redeemcode.NewService(repository, nil)
	remark := "活动赠送,\"客户补偿\"\n第二行"
	for i, expected := range []*string{&remark, nil} {
		input := redeemcode.GenerateInput{RequestID: "00000000-0000-4000-8000-000000000001", AmountCents: 100, Quantity: 2}
		if expected != nil {
			input.Remark = *expected
		} else {
			input.RequestID = "00000000-0000-4000-8000-000000000002"
		}
		result, err := service.Generate(ctx, 7, input)
		if err != nil {
			t.Fatal(err)
		}
		rows, count, err := repository.List(ctx, redeemcode.ListQuery{Page: 1, PageSize: 20, BatchID: result.BatchID}, time.Now())
		if err != nil || count != 2 || len(rows) != 2 {
			t.Fatalf("batch %d list: count=%d rows=%d error=%v", i, count, len(rows), err)
		}
		assertRemark := func(label string, value any) {
			t.Helper()
			data, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			var dto map[string]json.RawMessage
			if err := json.Unmarshal(data, &dto); err != nil {
				t.Fatal(err)
			}
			want, err := json.Marshal(expected)
			if err != nil || string(dto["remark"]) != string(want) {
				t.Fatalf("%s remark=%s, want=%s error=%v", label, dto["remark"], want, err)
			}
		}
		list, err := service.List(ctx, redeemcode.ListQuery{Page: 1, PageSize: 20, BatchID: result.BatchID})
		if err != nil || len(list.List) != 2 {
			t.Fatal("projected list missing", err)
		}
		for _, value := range list.List {
			assertRemark("list", value)
		}
		for _, row := range rows {
			value, err := service.Get(ctx, row.ID)
			if err != nil {
				t.Fatal(err)
			}
			assertRemark("detail", value)
			value, err = service.Lookup(ctx, row.Code)
			if err != nil {
				t.Fatal(err)
			}
			assertRemark("lookup", value)
		}
		exported, err := service.Export(ctx, []int64{rows[0].ID, rows[1].ID})
		if err != nil || len(exported) != 2 {
			t.Fatal("export missing", err)
		}
		for _, value := range exported {
			assertRemark("export", value)
		}
	}
}
