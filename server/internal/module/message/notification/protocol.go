package notification

import (
	"errors"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Variant string
type Priority string
type LinkType string
type AudienceType string

const (
	VariantInfo    Variant = "info"
	VariantSuccess Variant = "success"
	VariantWarning Variant = "warning"
	VariantError   Variant = "error"

	PriorityNormal Priority = "normal"
	PriorityUrgent Priority = "urgent"

	LinkNone     LinkType = "none"
	LinkInternal LinkType = "internal"
	LinkExternal LinkType = "external"

	AudienceTargeted AudienceType = "targeted"
	AudiencePlatform AudienceType = "platform"
)

type Content struct {
	Title       string
	ContentHTML string
	Summary     string
	Variant     Variant
	Priority    Priority
	LinkType    LinkType
	Link        string
}

func ValidateContent(content Content) error {
	if strings.TrimSpace(content.Title) == "" || utf8.RuneCountInString(content.Title) > 128 {
		return errors.New("notification title must contain 1..128 characters")
	}
	if strings.TrimSpace(content.ContentHTML) == "" || utf8.RuneCountInString(content.ContentHTML) > 16384 {
		return errors.New("notification content must contain 1..16384 characters")
	}
	if strings.TrimSpace(content.Summary) == "" || utf8.RuneCountInString(content.Summary) > 256 {
		return errors.New("notification summary must contain 1..256 characters")
	}
	switch content.Variant {
	case VariantInfo, VariantSuccess, VariantWarning, VariantError:
	default:
		return errors.New("notification variant is invalid")
	}
	switch content.Priority {
	case PriorityNormal, PriorityUrgent:
	default:
		return errors.New("notification priority is invalid")
	}
	return ValidateLink(content.LinkType, content.Link)
}

func ValidateLink(linkType LinkType, link string) error {
	if strings.IndexFunc(link, unicode.IsControl) >= 0 || strings.Contains(link, `\`) || strings.TrimSpace(link) != link || len(link) > 2048 {
		return errors.New("notification link contains invalid characters")
	}
	switch linkType {
	case LinkNone:
		if link != "" {
			return errors.New("notification none link must be empty")
		}
		return nil
	case LinkInternal:
		if link == "" || !strings.HasPrefix(link, "/") || strings.HasPrefix(link, "//") {
			return errors.New("notification internal link is invalid")
		}
		parsed, err := url.ParseRequestURI(link)
		if err != nil || parsed.IsAbs() || parsed.Host != "" {
			return errors.New("notification internal link is invalid")
		}
		return nil
	case LinkExternal:
		parsed, err := url.ParseRequestURI(link)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || !parsed.IsAbs() {
			return errors.New("notification external link is invalid")
		}
		return nil
	default:
		return errors.New("notification link type is invalid")
	}
}
