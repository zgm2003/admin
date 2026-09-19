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
	mu         sync.Mutex
	counts     map[string]int
	statements []string
	total      int
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
	if err == nil && isReadQuery(sql) {
		c.state.mu.Lock()
		c.state.total++
		c.state.statements = append(c.state.statements, strings.TrimSpace(sql))
		for table, matcher := range c.matchers {
			if matcher.MatchString(sql) {
				c.state.counts[table]++
			}
		}
		c.state.mu.Unlock()
	}
	c.Interface.Trace(ctx, begin, func() (string, int64) { return sql, rows }, err)
}

func isReadQuery(sql string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(sql))
	return strings.HasPrefix(normalized, "SELECT ") || strings.HasPrefix(normalized, "WITH ")
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

func (c *Counter) Statements() []string {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	return append([]string(nil), c.state.statements...)
}

func (c *Counter) Reset() {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	c.state.total = 0
	c.state.statements = nil
	for table := range c.state.counts {
		c.state.counts[table] = 0
	}
}
