package tool

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestHTTPTransportRejectsNonPublicDestinations(t *testing.T) {
	for _, raw := range []string{"https://127.0.0.1/x", "https://[::1]/x", "https://169.254.169.254/x", "https://10.0.0.1/x", "https://100.100.100.200/x", "https://example.com:444/x", "http://example.com/x", "https://example.com/x?auth=y"} {
		d := []byte(`{"url":"` + raw + `","method":"POST"}`)
		if e := safeHTTP(d); e == nil {
			t.Fatalf("restricted target accepted %s", raw)
		}
	}
	c := NewExecutor(nil, nil).http.(*http.Client)
	if c.CheckRedirect == nil {
		t.Fatal("redirect policy missing")
	}
	r := &http.Request{URL: &url.URL{Scheme: "https", Host: "example.com"}}
	if err := c.CheckRedirect(r, []*http.Request{r}); err == nil {
		t.Fatal("redirect accepted")
	}
	for _, raw := range []string{"127.0.0.1", "10.0.0.1", "::1", "fe80::1", "100.64.0.1", "224.0.0.1"} {
		if publicIP(net.ParseIP(raw)) {
			t.Fatalf("nonpublic IP accepted %s", raw)
		}
	}
	_ = context.Background()
	_ = strings.Builder{}
}
