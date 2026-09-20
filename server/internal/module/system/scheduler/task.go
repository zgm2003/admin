package scheduler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

func DecodeJobEnvelope(raw []byte) (JobEnvelope, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var envelope JobEnvelope
	if err := decoder.Decode(&envelope); err != nil {
		return JobEnvelope{}, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return JobEnvelope{}, errors.New("scheduler envelope must contain one JSON document")
	}
	if envelope.SchemaVersion != 1 || envelope.JobID <= 0 || envelope.Attempt <= 0 || envelope.DispatchToken == "" {
		return JobEnvelope{}, errors.New("scheduler envelope is invalid")
	}
	return envelope, nil
}
