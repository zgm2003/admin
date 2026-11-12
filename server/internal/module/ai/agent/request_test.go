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
	scene := int16(SceneToolGeneration)
	modelID := int64(9)
	agentType := int16(1)
	status := yesno.No
	input, err := (createRequest{
		Name: &name, Description: &description, SystemPrompt: &prompt,
		Scene: &scene, Type: &agentType,
		ProviderModelID: &modelID, Status: &status,
	}).input()
	if err != nil {
		t.Fatal(err)
	}
	if input.Name != "Support" || input.Description != "" ||
		input.SystemPrompt != "You are helpful." || input.Scene != SceneToolGeneration ||
		input.AgentType != 1 {
		t.Fatalf("input = %+v", input)
	}
}

func TestCreateRequestRejectsMissingPrompt(t *testing.T) {
	name := "Support"
	description := ""
	prompt := "   "
	scene := int16(SceneConversation)
	modelID := int64(9)
	agentType := int16(1)
	status := yesno.No
	if _, err := (createRequest{
		Name: &name, Description: &description, SystemPrompt: &prompt,
		Scene: &scene, Type: &agentType,
		ProviderModelID: &modelID, Status: &status,
	}).input(); err == nil {
		t.Fatal("empty prompt was accepted")
	}
}

func TestCreateRequestRejectsInvalidTypeAndScene(t *testing.T) {
	name := "Support"
	description := ""
	prompt := "help"
	invalidScene := int16(9)
	validScene := int16(SceneConversation)
	modelID := int64(9)
	invalidType := int16(9)
	validType := int16(1)
	status := yesno.No
	if _, err := (createRequest{
		Name: &name, Description: &description, SystemPrompt: &prompt,
		Scene: &invalidScene, Type: &validType,
		ProviderModelID: &modelID, Status: &status,
	}).input(); err == nil {
		t.Fatal("invalid scene was accepted")
	}
	if _, err := (createRequest{
		Name: &name, Description: &description, SystemPrompt: &prompt,
		Scene: &validScene, Type: &invalidType,
		ProviderModelID: &modelID, Status: &status,
	}).input(); err == nil {
		t.Fatal("invalid type was accepted")
	}
}

func TestParseListQueryValidatesAllowlistAndStatus(t *testing.T) {
	query, err := parseListQuery(url.Values{
		"page": {"1"}, "pageSize": {"20"}, "keyword": {" support "},
		"type": {"2"}, "scene": {"3"}, "status": {"1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if query.Keyword != "support" || query.Type == nil || *query.Type != 2 ||
		query.Scene == nil || *query.Scene != SceneImageGeneration ||
		query.Status == nil || *query.Status != yesno.Yes {
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
	if _, err := parseListQuery(url.Values{
		"page": {"1"}, "pageSize": {"20"}, "type": {"9"},
	}); err == nil {
		t.Fatal("invalid type was accepted")
	}
	if _, err := parseListQuery(url.Values{
		"page": {"1"}, "pageSize": {"20"}, "scene": {"9"},
	}); err == nil {
		t.Fatal("invalid scene was accepted")
	}
}
