package run

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const maxTextHistoryTurns = 100
const maxTextProtocolBytes = 16 << 20

var ErrTextContextLimit = errors.New("AI text context exceeds frozen policy")

type textFileReader interface {
	ReadAIFrozenReference(context.Context, int64, int64, InputFileSnapshot, int64) ([]byte, string, string, error)
}

func (e *TextEngine) SetFileReader(r textFileReader) { e.files = r }
func (r *Repository) TextHistory(ctx context.Context, row Model, maxBytes int64) ([]historyTurn, error) {
	out := make([]historyTurn, 0)
	if row.ConversationID == nil {
		return out, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if row.UserID < 1 || row.PlatformID < 1 || maxBytes < 1 {
		return nil, ErrInvalidSnapshot
	}
	var meta []struct {
		ID    int64
		Bytes int64
	}
	e := r.db.WithContext(ctx).Raw(`SELECT m.id,octet_length(m.content::text)+CASE WHEN m.role=1 THEN coalesce(octet_length(prior.input_snapshot::text),0) ELSE 0 END AS bytes FROM ai_message m JOIN ai_conversation c ON c.id=m.conversation_id AND c.user_id=? AND c.platform_id=? JOIN ai_run prior ON prior.id=m.run_id AND prior.user_id=c.user_id AND prior.platform_id=c.platform_id WHERE m.conversation_id=? AND m.sequence<(SELECT min(sequence) FROM ai_message WHERE conversation_id=? AND run_id=? AND role=1) AND m.role IN(1,2) ORDER BY m.sequence DESC LIMIT ?`, row.UserID, row.PlatformID, *row.ConversationID, *row.ConversationID, row.ID, maxTextHistoryTurns+1).Scan(&meta).Error
	if e != nil {
		return nil, e
	}
	if len(meta) > maxTextHistoryTurns {
		return nil, ErrTextContextLimit
	}
	ids := make([]int64, 0, len(meta))
	total := int64(0)
	for _, m := range meta {
		if m.Bytes < 0 || m.Bytes > maxBytes-total {
			return nil, ErrTextContextLimit
		}
		total += m.Bytes
		ids = append(ids, m.ID)
	}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []struct {
		Sequence      int64
		Role          int16
		Content       json.RawMessage
		InputSnapshot json.RawMessage
	}
	e = r.db.WithContext(ctx).Table("ai_message m").Select("m.sequence,m.role,m.content,CASE WHEN m.role=1 THEN prior.input_snapshot ELSE '{}'::jsonb END AS input_snapshot").Joins("JOIN ai_run prior ON prior.id=m.run_id AND prior.user_id=? AND prior.platform_id=?", row.UserID, row.PlatformID).Where("m.id IN ? AND m.conversation_id=?", ids, *row.ConversationID).Order("m.sequence").Limit(maxTextHistoryTurns).Scan(&rows).Error
	if e != nil {
		return nil, e
	}
	if len(rows) != len(ids) {
		return nil, ErrInvalidSnapshot
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Sequence < rows[j].Sequence })
	for _, m := range rows {
		role := "user"
		if m.Role == 2 {
			role = "assistant"
		}
		turn := historyTurn{Role: role, Content: m.Content}
		if m.Role == 1 {
			var input RunInput
			if e = json.Unmarshal(m.InputSnapshot, &input); e != nil {
				return nil, e
			}
			turn.Files = input.FileSnapshots
		}
		out = append(out, turn)
	}
	return out, nil
}
func (e *TextEngine) prepareTextInput(ctx context.Context, row Model, protocol int16, history []historyTurn, current RunInput, policy Policy) ([]json.RawMessage, error) {
	turns := append(append([]historyTurn(nil), history...), historyTurn{Role: "user", Text: current.Text, Files: current.FileSnapshots})
	attachments := int64(0)
	for _, turn := range turns {
		attachments += int64(len(turn.Files))
	}
	if attachments > policy.MaxInputAttachments || attachments > 4 {
		return nil, ErrTextContextLimit
	}
	out := make([]json.RawMessage, 0, len(turns))
	totalBytes := 0
	for _, turn := range turns {
		if turn.Role != "user" && turn.Role != "assistant" {
			return nil, ErrInvalidSnapshot
		}
		texts := make([]string, 0)
		if turn.Content != nil {
			var parts []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}
			if err := json.Unmarshal(turn.Content, &parts); err != nil {
				return nil, err
			}
			for _, part := range parts {
				if part.Type != "input_text" && part.Type != "output_text" && part.Type != "text" {
					return nil, ErrInvalidSnapshot
				}
				texts = append(texts, part.Text)
			}
		} else {
			texts = append(texts, turn.Text)
		}
		content := make([]map[string]any, 0, len(texts)+len(turn.Files))
		for _, text := range texts {
			kind := "input_text"
			if protocol == 2 {
				kind = "text"
			} else if turn.Role == "assistant" {
				kind = "output_text"
			}
			content = append(content, map[string]any{"type": kind, "text": text})
		}
		for _, file := range turn.Files {
			if turn.Role != "user" || e.files == nil || !strings.HasPrefix(file.MIMEType, "image/") || file.SizeBytes < 1 || file.SizeBytes > policy.ImageMaxBytes {
				return nil, ErrInvalidSnapshot
			}
			if file.MIMEType != "image/png" && file.MIMEType != "image/jpeg" && file.MIMEType != "image/webp" && file.MIMEType != "image/gif" {
				return nil, ErrInvalidSnapshot
			}
			if file.SizeBytes > int64((maxTextProtocolBytes-totalBytes)*3/4) {
				return nil, ErrTextContextLimit
			}
			data, mime, _, err := e.files.ReadAIFrozenReference(ctx, row.UserID, row.PlatformID, file, policy.ImageMaxBytes)
			if err != nil {
				return nil, err
			}
			if int64(len(data)) != file.SizeBytes || mime != file.MIMEType {
				return nil, ErrInvalidSnapshot
			}
			source := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
			if protocol == 2 {
				content = append(content, map[string]any{"type": "image_url", "image_url": map[string]string{"url": source}})
			} else {
				content = append(content, map[string]any{"type": "input_image", "image_url": source})
			}
		}
		raw, err := json.Marshal(map[string]any{"role": turn.Role, "content": content})
		if err != nil {
			return nil, err
		}
		totalBytes += len(raw)
		if totalBytes > maxTextProtocolBytes {
			return nil, ErrTextContextLimit
		}
		out = append(out, raw)
	}
	return out, nil
}
func nativeContextBytes(items []json.RawMessage) (int64, error) {
	total := int64(0)
	for _, raw := range items {
		var value any
		if e := json.Unmarshal(raw, &value); e != nil {
			return 0, e
		}
		var count func(any)
		count = func(v any) {
			switch x := v.(type) {
			case map[string]any:
				for key, value := range x {
					if key == "image_url" || key == "url" {
						continue
					}
					count(value)
				}
			case []any:
				for _, value := range x {
					count(value)
				}
			case string:
				total += int64(len(x))
			}
		}
		count(value)
	}
	return total, nil
}
func textRequestSummary(native []json.RawMessage) (json.RawMessage, error) {
	raw, e := json.Marshal(native)
	if e != nil {
		return nil, e
	}
	sum := sha256.Sum256(raw)
	return json.Marshal(struct {
		InputDigest string `json:"inputDigest"`
		InputItems  int    `json:"inputItems"`
		InputBytes  int    `json:"inputBytes"`
	}{fmt.Sprintf("%x", sum), len(native), len(raw)})
}
