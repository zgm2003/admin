package queue

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
)

type Client struct {
	client *asynq.Client
}

func NewClient(redisURL string) (*Client, error) {
	redisOptions, err := asynq.ParseRedisURI(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse Redis URL for Asynq client: %w", err)
	}
	return &Client{client: asynq.NewClient(redisOptions)}, nil
}

func (c *Client) Enqueue(ctx context.Context, task *asynq.Task, options ...asynq.Option) (*asynq.TaskInfo, error) {
	info, err := c.client.EnqueueContext(ctx, task, options...)
	if err != nil {
		return nil, fmt.Errorf("enqueue Asynq task: %w", err)
	}
	return info, nil
}

func (c *Client) Close() error {
	return c.client.Close()
}
