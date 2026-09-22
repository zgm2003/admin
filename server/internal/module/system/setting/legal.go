package setting

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const maxLegalDocumentRunes = 65535

var legalHTTPSHref = regexp.MustCompile(`(?i)^https://[^\s]+$`)

func normalizeLegalDocumentHTML(raw string) (string, error) {
	policy := bluemonday.NewPolicy()
	policy.AllowElements("p", "br", "strong", "em", "u", "h2", "h3", "ul", "ol", "li", "a")
	policy.AllowAttrs("href").Matching(legalHTTPSHref).OnElements("a")
	cleaned := policy.Sanitize(strings.TrimSpace(raw))
	hardened, text, err := hardenLegalDocumentHTML(cleaned)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(text) == "" {
		return "", errors.New("legal document content is empty after sanitizing")
	}
	if utf8.RuneCountInString(hardened) > maxLegalDocumentRunes {
		return "", errors.New("legal document content is too long")
	}
	return hardened, nil
}

func hardenLegalDocumentHTML(content string) (string, string, error) {
	contextNode := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	nodes, err := html.ParseFragment(strings.NewReader(content), contextNode)
	if err != nil {
		return "", "", errors.New("parse sanitized legal document content")
	}
	var text strings.Builder
	for _, node := range nodes {
		walkLegalDocumentNodes(node, func(current *html.Node) {
			if current.Type == html.TextNode {
				text.WriteString(current.Data)
				text.WriteByte(' ')
			}
			if current.Type != html.ElementNode || current.Data != "a" {
				return
			}
			href := legalDocumentAttribute(current, "href")
			current.Attr = nil
			if legalHTTPSHref.MatchString(href) {
				current.Attr = []html.Attribute{
					{Key: "href", Val: href},
					{Key: "target", Val: "_blank"},
					{Key: "rel", Val: "noopener noreferrer"},
				}
			}
		})
	}
	var output bytes.Buffer
	for _, node := range nodes {
		if err := html.Render(&output, node); err != nil {
			return "", "", errors.New("render sanitized legal document content")
		}
	}
	return output.String(), text.String(), nil
}

func walkLegalDocumentNodes(node *html.Node, visit func(*html.Node)) {
	visit(node)
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		walkLegalDocumentNodes(child, visit)
	}
}

func legalDocumentAttribute(node *html.Node, name string) string {
	for _, attribute := range node.Attr {
		if attribute.Key == name {
			return attribute.Val
		}
	}
	return ""
}
