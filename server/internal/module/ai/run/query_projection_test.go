package run

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPublicProviderProjectionDropsNativeToolsAndUpstreamSources(t *testing.T) {
	in := json.RawMessage(`[{"type":"function_call","arguments":"secret"},{"url":"https://upstream/TOKEN"},{"type":"message","content":[{"type":"output_text","text":"answer","annotations":[{"token":"secret"}]}]}]`)
	out, e := publicProviderItems(in)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(out), "secret") || strings.Contains(string(out), "upstream") || !strings.Contains(string(out), "answer") {
		t.Fatal(string(out))
	}
	usage, e := publicProviderUsage(json.RawMessage(`{"known":true,"inputTokens":2,"raw":{"authorization":"secret"}}`))
	if e != nil || strings.Contains(string(usage), "secret") {
		t.Fatalf("%s %v", usage, e)
	}
}
