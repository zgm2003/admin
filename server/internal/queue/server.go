package queue

import (
	"fmt"

	"github.com/hibiken/asynq"
)

func NewServer(redisURL string) (*asynq.Server, error) {
	redisOptions, err := asynq.ParseRedisURI(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse Redis URL for Asynq server: %w", err)
	}
	return asynq.NewServer(redisOptions, asynq.Config{Concurrency: 10}), nil
}
