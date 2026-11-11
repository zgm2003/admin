package agent

import (
	"net/url"
	"testing"

	"admin/server/internal/shared/yesno"
)

func TestCreateRequestRequiresAndTrimsFields(t *testing.T) {
	name := "  Support  "
	description := "  "
	prompt := "  You are helpful.  "
	modelID := int64(9)
	status := yesno.No
	input, err := (createRequest{
		Name: &name, Description: &description, SystemPrompt: &prompt,
		ProviderModelID: &modelID, Status: &status,
	}).input()
	if err != nil {
		t.Fatal(err)
	}
	if input.Name != "Support" || input.Description != "" || input.SystemPrompt != "You are helpful." {
		t.Fatalf("input = %+v", input)
	}
}

func TestCreateRequestRejectsMissingPrompt(t *testing.T) {
	name := "Support"
	description := ""
	prompt := "   "
	modelID := int64(9)
	status := yesno.No
	if _, err := (createRequest{
		Name: &name, Description: &description, SystemPrompt: &prompt,
		ProviderModelID: &modelID, Status: &status,
	}).input(); err == nil {
		t.Fatal("empty prompt was accepted")
	}
}

func TestParseListQueryValidatesAllowlistAndStatus(t *testing.T) {
	query, err := parseListQuery(url.Values{
		"page": {"1"}, "pageSize": {"20"}, "keyword": {" support "}, "status": {"1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if query.Keyword != "support" || query.Status == nil || *query.Status != yesno.Yes {
		t.Fatalf("query = %+v", query)
	}
	if _, err := parseListQuery(url.Values{
		"page": {"1"}, "pageSize": {"20"}, "unknown": {"x"},
	}); err == nil {
		t.Fatal("unknown query was accepted")
	}
	if _, err := parseListQuery(url.Values{
		"page": {"1"}, "pageSize": {"20"}, "status": {"9"},
	}); err == nil {
		t.Fatal("invalid status was accepted")
	}
}
