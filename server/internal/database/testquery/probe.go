// Package testquery provides a test-only GORM SQL budget probe.
package testquery

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm/logger"
)

// Counter records successful SELECT statements, including GORM Raw queries.
type Counter struct {
	logger.Interface
	matchers map[string]*regexp.Regexp
	state    *counterState
}

type counterState struct {
	mu     sync.Mutex
	counts map[string]int
	total  int
}

func New(base logger.Interface, tables ...string) *Counter {
	counts := make(map[string]int, len(tables))
	matchers := make(map[string]*regexp.Regexp, len(tables))
	for _, table := range tables {
		counts[table] = 0
		matchers[table] = regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(table) + `\b`)
	}
	return &Counter{Interface: base, matchers: matchers, state: &counterState{counts: counts}}
}

func (c *Counter) LogMode(level logger.LogLevel) logger.Interface {
	return &Counter{Interface: c.Interface.LogMode(level), matchers: c.matchers, state: c.state}
}

func (c *Counter) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	sql, rows := fc()
	if err == nil && strings.HasPrefix(strings.ToUpper(strings.TrimSpace(sql)), "SELECT ") {
		c.state.mu.Lock()
		c.state.total++
		for table, matcher := range c.matchers {
			if matcher.MatchString(sql) {
				c.state.counts[table]++
			}
		}
		c.state.mu.Unlock()
	}
	c.Interface.Trace(ctx, begin, func() (string, int64) { return sql, rows }, err)
}

func (c *Counter) Count(table string) int {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	return c.state.counts[table]
}

func (c *Counter) Total() int {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	return c.state.total
}

func (c *Counter) Reset() {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	c.state.total = 0
	for table := range c.state.counts {
		c.state.counts[table] = 0
	}
}
