package run

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"admin/server/internal/module/ai/chat"
)

func TestSearchBingRSSParsesAndDeduplicates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("format") != "rss" || r.URL.Query().Get("q") != "Go 语言" {
			t.Fatalf("query = %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<rss><channel>
  <item><title>Go 官网</title><link>https://go.dev/</link><description>&lt;b&gt;The Go&lt;/b&gt; programming language</description></item>
  <item><title>重复</title><link>https://go.dev/</link><description>duplicate</description></item>
  <item><title>无效链接</title><link>javascript:alert(1)</link><description>bad</description></item>
  <item><title>文档</title><link>https://pkg.go.dev/</link><description>Packages</description></item>
</channel></rss>`))
	}))
	defer server.Close()

	oldEndpoint, oldClient := bingSearchEndpoint, webSearchClient
	bingSearchEndpoint, webSearchClient = server.URL, server.Client()
	defer func() {
		bingSearchEndpoint, webSearchClient = oldEndpoint, oldClient
	}()

	results, err := searchBingRSS(context.Background(), "Go 语言", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %#v", results)
	}
	if results[0].Index != 1 || results[0].Title != "Go 官网" || results[0].URL != "https://go.dev/" {
		t.Fatalf("first result = %#v", results[0])
	}
	if results[0].Snippet != "The Go programming language" {
		t.Fatalf("snippet = %q", results[0].Snippet)
	}
	if results[1].Index != 2 || results[1].URL != "https://pkg.go.dev/" {
		t.Fatalf("second result = %#v", results[1])
	}
}

func TestParseBraveResults(t *testing.T) {
	page := `<div class="snippet svelte-a" data-pos="0" data-type="web" data-keynav="true">
<a href="https://zh.wikipedia.org/wiki/美国总统">...<div class="title search-snippet-title line-clamp-1">美国总统 - 维基百科</div></a>
<div class="generic-snippet"><div class="content desktop-default-regular t-primary">美利坚合众国总统是美国的国家元首。</div></div></div>
<div class="snippet svelte-a" data-pos="1" data-type="web">
<a href="https://www.whitehouse.gov/">...<div class="title search-snippet-title">The White House</div></a>
<div class="generic-snippet"><div class="content desktop-default-regular">President news.</div></div></div>`
	results := parseBraveResults(page, 5)
	if len(results) != 2 {
		t.Fatalf("results = %#v", results)
	}
	if results[0].Title != "美国总统 - 维基百科" || results[0].URL != "https://zh.wikipedia.org/wiki/美国总统" {
		t.Fatalf("first = %#v", results[0])
	}
	if results[0].Snippet != "美利坚合众国总统是美国的国家元首。" {
		t.Fatalf("snippet = %q", results[0].Snippet)
	}
}

func TestNormalizeSearchQuery(t *testing.T) {
	cases := map[string]string{
		"查一下现在的美国总统是谁？":             "美国总统",
		"帮我搜索 OpenAI Responses API": "OpenAI Responses API",
		"Go 语言是什么":                  "Go 语言",
		"现任美国总统":                    "现任美国总统",
	}
	for input, want := range cases {
		if got := normalizeSearchQuery(input); got != want {
			t.Fatalf("normalize(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSearchWebFallsBackToBing(t *testing.T) {
	brave := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer brave.Close()
	bing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<rss><channel><item><title>OpenAI</title><link>https://openai.com/</link><description>AI company</description></item></channel></rss>`))
	}))
	defer bing.Close()

	oldBrave, oldBing, oldClient := braveSearchEndpoint, bingSearchEndpoint, webSearchClient
	braveSearchEndpoint, bingSearchEndpoint, webSearchClient = brave.URL, bing.URL, brave.Client()
	defer func() {
		braveSearchEndpoint, bingSearchEndpoint, webSearchClient = oldBrave, oldBing, oldClient
	}()

	results, err := SearchWeb(context.Background(), "OpenAI", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Title != "OpenAI" {
		t.Fatalf("results = %#v", results)
	}
}

func TestSearchSystemPrompt(t *testing.T) {
	prompt := searchSystemPrompt("You are helpful.", []chat.Citation{})
	if !strings.Contains(prompt, "没有获得可用结果") || !strings.Contains(prompt, "禁止回答") {
		t.Fatalf("prompt = %q", prompt)
	}
}
