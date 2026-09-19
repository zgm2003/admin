package notification

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

var httpsHref = regexp.MustCompile(`(?i)^https://[^\s]+$`)

func SanitizeContent(raw string) (string, error) {
	policy := bluemonday.NewPolicy()
	policy.AllowElements("p", "br", "strong", "em", "u", "h2", "h3", "ul", "ol", "li", "a")
	policy.AllowAttrs("href").Matching(httpsHref).OnElements("a")
	cleaned := policy.Sanitize(raw)
	hardened, err := hardenAnchors(cleaned)
	if err != nil {
		return "", err
	}
	if utf8.RuneCountInString(hardened) > 16384 {
		return "", errors.New("notification content exceeds 16384 characters")
	}
	summary, err := SummaryFromHTML(hardened)
	if err != nil {
		return "", err
	}
	if summary == "" {
		return "", errors.New("notification content is empty after sanitizing")
	}
	return hardened, nil
}

func SummaryFromHTML(content string) (string, error) {
	nodes, err := html.ParseFragment(strings.NewReader(content), &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div})
	if err != nil {
		return "", errors.New("parse notification content")
	}
	var text strings.Builder
	for _, node := range nodes {
		appendVisibleText(&text, node)
	}
	collapsed := strings.Join(strings.Fields(text.String()), " ")
	runes := []rune(collapsed)
	if len(runes) > 256 {
		runes = runes[:256]
	}
	return string(runes), nil
}

func hardenAnchors(content string) (string, error) {
	contextNode := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	nodes, err := html.ParseFragment(strings.NewReader(content), contextNode)
	if err != nil {
		return "", errors.New("parse sanitized notification content")
	}
	for _, node := range nodes {
		walkNodes(node, func(current *html.Node) {
			if current.Type != html.ElementNode || current.Data != "a" {
				return
			}
			href := attribute(current, "href")
			current.Attr = nil
			if ValidateLink(LinkExternal, href) == nil {
				current.Attr = []html.Attribute{{Key: "href", Val: href}, {Key: "target", Val: "_blank"}, {Key: "rel", Val: "noopener noreferrer"}}
			}
		})
	}
	var output bytes.Buffer
	for _, node := range nodes {
		if err := html.Render(&output, node); err != nil {
			return "", errors.New("render sanitized notification content")
		}
	}
	return output.String(), nil
}

func walkNodes(node *html.Node, visit func(*html.Node)) {
	visit(node)
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		walkNodes(child, visit)
	}
}

func attribute(node *html.Node, name string) string {
	for _, attribute := range node.Attr {
		if attribute.Key == name {
			return attribute.Val
		}
	}
	return ""
}

func appendVisibleText(output *strings.Builder, node *html.Node) {
	if node.Type == html.TextNode {
		output.WriteString(node.Data)
		output.WriteByte(' ')
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		appendVisibleText(output, child)
	}
}
