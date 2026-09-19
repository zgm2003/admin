package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	auth "admin/server/internal/module/auth/login"
	"github.com/google/uuid"
)

type Service struct {
	tickets     *TicketStore
	sessions    SessionValidator
	repository  *Repository
	resumeSlots chan struct{}
	now         func() time.Time
}

func NewService(tickets *TicketStore, sessions SessionValidator, repository *Repository, resumeConcurrency int) *Service {
	if resumeConcurrency < 1 {
		resumeConcurrency = 32
	}
	return &Service{tickets: tickets, sessions: sessions, repository: repository, resumeSlots: make(chan struct{}, resumeConcurrency), now: time.Now}
}

func (s *Service) IssueTicket(ctx context.Context, identity auth.Identity) (string, time.Time, error) {
	now := s.now().UTC()
	expiresAt := now.Add(30 * time.Second)
	if identity.AccessExpiresAt.Before(expiresAt) {
		expiresAt = identity.AccessExpiresAt.UTC()
	}
	raw, err := s.tickets.Issue(ctx, TicketSubject{SchemaVersion: 1, PlatformID: identity.PlatformID, PlatformCode: identity.Platform, UserID: identity.UserID, SessionID: identity.SessionID, SessionVersion: identity.Version, AccessExpiresAt: identity.AccessExpiresAt.UTC(), IssuedAt: now})
	return raw, expiresAt, err
}
func (s *Service) ConsumeTicket(ctx context.Context, raw string) (TicketSubject, error) {
	subject, err := s.tickets.Consume(ctx, raw)
	if err != nil {
		return TicketSubject{}, err
	}
	if err := s.sessions.ValidateRealtimeSession(ctx, subject.PlatformCode, subject.UserID, subject.SessionID, subject.SessionVersion); err != nil {
		return TicketSubject{}, err
	}
	return subject, nil
}

func (s *Service) Resume(ctx context.Context, subject TicketSubject, after int64, requestID string) ([][]byte, error) {
	select {
	case s.resumeSlots <- struct{}{}:
		defer func() { <-s.resumeSlots }()
	default:
		return nil, errors.New("resume concurrency limit reached")
	}
	resumeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	window, err := s.repository.ResumeWindow(resumeCtx, subject.PlatformID, subject.UserID, after, 500)
	if err != nil {
		return nil, err
	}
	through := window.ThroughSequence
	if through < 1 {
		through = 1
	}
	request := requestID
	if window.ResyncRequired {
		data, _ := json.Marshal(struct {
			ThroughSequence int64 `json:"throughSequence"`
		}{window.ThroughSequence})
		payload, err := EncodeEnvelope(Envelope{EventID: uuid.NewString(), Type: EventRealtimeResyncRequired, RequestID: &request, Sequence: through, OccurredAt: s.now().UTC(), Durability: DurabilityDurable, Data: data})
		return [][]byte{payload}, err
	}
	result := make([][]byte, 0, len(window.Events)+1)
	for _, event := range window.Events {
		payload, err := EncodeEnvelope(Envelope{EventID: event.EventID, Type: event.EventType, Sequence: event.Sequence, OccurredAt: event.OccurredAt.UTC(), Durability: DurabilityDurable, Data: event.Payload})
		if err != nil {
			return nil, err
		}
		result = append(result, payload)
	}
	data, _ := json.Marshal(struct {
		ThroughSequence int64 `json:"throughSequence"`
	}{window.ThroughSequence})
	completed, err := EncodeEnvelope(Envelope{EventID: uuid.NewString(), Type: EventRealtimeResumed, RequestID: &request, Sequence: through, OccurredAt: s.now().UTC(), Durability: DurabilityDurable, Data: data})
	if err != nil {
		return nil, err
	}
	return append(result, completed), nil
}
