package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type HTTPDefinition struct {
	URL             string            `json:"url"`
	Method          string            `json:"method"`
	Headers         map[string]string `json:"headers,omitempty"`
	RequestMapping  map[string]string `json:"requestMapping"`
	ResponseMapping map[string]string `json:"responseMapping"`
}

func parseHTTPDefinition(raw json.RawMessage) (HTTPDefinition, error) {
	var d HTTPDefinition
	if len(raw) > 16384 || !json.Valid(raw) {
		return d, errors.New("HTTP definition invalid")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if e := dec.Decode(&d); e != nil {
		return d, e
	}
	if d.URL == "" || len(d.URL) > 2048 {
		return d, errors.New("HTTP URL required")
	}
	u, e := url.Parse(d.URL)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Port() != "" && u.Port() != "443") {
		return d, errors.New("HTTP URL must be fixed public HTTPS on port 443")
	}
	h := strings.ToLower(u.Hostname())
	if strings.ContainsAny(h, "{}% ") || h == "localhost" || strings.HasSuffix(h, ".localhost") || strings.HasSuffix(h, ".internal") {
		return d, errors.New("HTTP host invalid")
	}
	if ip := net.ParseIP(h); ip != nil && !publicIP(ip) {
		return d, errors.New("HTTP private destination is forbidden")
	}
	if d.Method == "" {
		d.Method = "POST"
	}
	switch d.Method {
	case "GET", "POST", "PUT", "PATCH", "DELETE":
	default:
		return d, errors.New("HTTP method invalid")
	}
	if len(d.Headers) > 16 || len(d.RequestMapping) > 128 || len(d.ResponseMapping) > 128 {
		return d, errors.New("HTTP mapping limit exceeded")
	}
	for k, v := range d.Headers {
		if !allowedHeader(k) || len(v) > 4096 || strings.ContainsAny(v, "\r\n") || strings.EqualFold(k, "authorization") || strings.Contains(strings.ToLower(k), "key") || strings.EqualFold(k, "cookie") {
			return d, errors.New("HTTP static header invalid; authentication belongs in secretHeaders")
		}
	}
	for _, m := range []map[string]string{d.RequestMapping, d.ResponseMapping} {
		for k, v := range m {
			if k == "" || v == "" || len(k) > 128 || len(v) > 128 {
				return d, errors.New("HTTP mapping invalid")
			}
		}
	}
	return d, nil
}
func allowedHeader(k string) bool {
	if k == "" || len(k) > 128 {
		return false
	}
	for _, c := range k {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-') {
			return false
		}
	}
	switch strings.ToLower(k) {
	case "host", "content-length", "connection", "transfer-encoding", "proxy-authorization", "proxy-connection", "upgrade", "te", "trailer":
		return false
	}
	return true
}
func newHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, e := net.SplitHostPort(address)
		if e != nil || port != "443" {
			return nil, errors.New("HTTP tool destination port is forbidden")
		}
		addresses, e := net.DefaultResolver.LookupIPAddr(ctx, host)
		if e != nil {
			return nil, e
		}
		if len(addresses) == 0 {
			return nil, errors.New("HTTP destination has no addresses")
		}
		for _, a := range addresses {
			if !publicIP(a.IP) {
				return nil, errors.New("HTTP destination resolved to a non-public IP")
			}
		}
		var last error
		for _, a := range addresses {
			c, e := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(a.IP.String(), port))
			if e == nil {
				return c, nil
			}
			last = e
		}
		return nil, fmt.Errorf("HTTP tool connection failed: %w", last)
	}
	return &http.Client{Transport: transport, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return errors.New("HTTP redirects are forbidden") }}
}
