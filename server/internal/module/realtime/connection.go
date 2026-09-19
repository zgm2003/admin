package realtime

import (
	"errors"
	"fmt"
	"sync"
)

const CloseSlowConsumer = 4008

var (
	ErrConnectionLimit = errors.New("realtime connection limit reached")
	ErrDraining        = errors.New("realtime connections are draining")
)

type Connection struct {
	PlatformID   int64
	PlatformCode string
	UserID       int64
	SessionID    int64
	send         chan []byte
	closeOnce    sync.Once
	closeFn      func(int, string)
	mu           sync.Mutex
	closed       bool
	closeCode    int
}

func NewConnection(platformID, userID, sessionID int64, capacity int, closeFn func(int, string)) *Connection {
	if capacity < 1 {
		capacity = 128
	}
	return &Connection{PlatformID: platformID, UserID: userID, SessionID: sessionID, send: make(chan []byte, capacity), closeFn: closeFn}
}
func (c *Connection) Send() <-chan []byte { return c.send }
func (c *Connection) enqueue(payload []byte) bool {
	select {
	case c.send <- append([]byte(nil), payload...):
		return true
	default:
		return false
	}
}
func (c *Connection) Close(code int, reason string) {
	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.closed = true
		c.closeCode = code
		c.mu.Unlock()
		if c.closeFn != nil {
			c.closeFn(code, reason)
		}
	})
}
func (c *Connection) Closed() bool { c.mu.Lock(); defer c.mu.Unlock(); return c.closed }
func (c *Connection) ClosedWith(code int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed && c.closeCode == code
}

type sessionConnectionKey struct{ platformID, sessionID int64 }
type userConnectionKey struct{ platformID, userID int64 }

type ConnectionSet struct {
	mu                         sync.RWMutex
	maxConnections, maxPerUser int
	draining                   bool
	bySession                  map[sessionConnectionKey]*Connection
	byUser                     map[userConnectionKey]map[*Connection]struct{}
	byPlatform                 map[int64]map[*Connection]struct{}
	subscriptionObserver       func(DesiredSubscriptions)
}

func NewConnectionSet(maxConnections, maxPerUser int) *ConnectionSet {
	return &ConnectionSet{maxConnections: maxConnections, maxPerUser: maxPerUser, bySession: map[sessionConnectionKey]*Connection{}, byUser: map[userConnectionKey]map[*Connection]struct{}{}, byPlatform: map[int64]map[*Connection]struct{}{}}
}
func (s *ConnectionSet) Attach(connection *Connection) error {
	if connection == nil || connection.PlatformID <= 0 || connection.UserID <= 0 || connection.SessionID <= 0 {
		return errors.New("invalid realtime connection")
	}
	var replaced *Connection
	s.mu.Lock()
	if s.draining {
		s.mu.Unlock()
		return ErrDraining
	}
	sk := sessionConnectionKey{connection.PlatformID, connection.SessionID}
	replaced = s.bySession[sk]
	count := len(s.bySession)
	if replaced != nil {
		count--
	}
	uk := userConnectionKey{connection.PlatformID, connection.UserID}
	userCount := len(s.byUser[uk])
	if replaced != nil && replaced.UserID == connection.UserID {
		userCount--
	}
	if count >= s.maxConnections || userCount >= s.maxPerUser {
		s.mu.Unlock()
		return ErrConnectionLimit
	}
	if replaced != nil {
		s.detachLocked(replaced)
	}
	s.bySession[sk] = connection
	if s.byUser[uk] == nil {
		s.byUser[uk] = map[*Connection]struct{}{}
	}
	s.byUser[uk][connection] = struct{}{}
	if s.byPlatform[connection.PlatformID] == nil {
		s.byPlatform[connection.PlatformID] = map[*Connection]struct{}{}
	}
	s.byPlatform[connection.PlatformID][connection] = struct{}{}
	snapshot, observer := s.subscriptionSnapshotLocked()
	s.mu.Unlock()
	if observer != nil {
		observer(snapshot)
	}
	if replaced != nil {
		replaced.Close(4000, "replaced")
	}
	return nil
}
func (s *ConnectionSet) Detach(connection *Connection) {
	s.mu.Lock()
	s.detachLocked(connection)
	snapshot, observer := s.subscriptionSnapshotLocked()
	s.mu.Unlock()
	if observer != nil {
		observer(snapshot)
	}
}
func (s *ConnectionSet) detachLocked(c *Connection) {
	if c == nil {
		return
	}
	sk := sessionConnectionKey{c.PlatformID, c.SessionID}
	if s.bySession[sk] != c {
		return
	}
	delete(s.bySession, sk)
	uk := userConnectionKey{c.PlatformID, c.UserID}
	delete(s.byUser[uk], c)
	if len(s.byUser[uk]) == 0 {
		delete(s.byUser, uk)
	}
	delete(s.byPlatform[c.PlatformID], c)
	if len(s.byPlatform[c.PlatformID]) == 0 {
		delete(s.byPlatform, c.PlatformID)
	}
}
func (s *ConnectionSet) Len() int { s.mu.RLock(); defer s.mu.RUnlock(); return len(s.bySession) }
func (s *ConnectionSet) PublishUser(platformID, userID int64, payload []byte) {
	s.publish(s.snapshotUser(platformID, userID), payload)
}
func (s *ConnectionSet) PublishPlatform(platformID, audienceMaxUserID int64, payload []byte) {
	targets := s.snapshotPlatform(platformID)
	filtered := targets[:0]
	for _, c := range targets {
		if c.UserID <= audienceMaxUserID {
			filtered = append(filtered, c)
		}
	}
	s.publish(filtered, payload)
}
func (s *ConnectionSet) publish(targets []*Connection, payload []byte) {
	for _, c := range targets {
		if !c.enqueue(payload) {
			s.Detach(c)
			c.Close(CloseSlowConsumer, "slow consumer")
		}
	}
}
func (s *ConnectionSet) snapshotUser(platformID, userID int64) []*Connection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copyConnections(s.byUser[userConnectionKey{platformID, userID}])
}
func (s *ConnectionSet) snapshotPlatform(platformID int64) []*Connection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copyConnections(s.byPlatform[platformID])
}
func copyConnections(values map[*Connection]struct{}) []*Connection {
	result := make([]*Connection, 0, len(values))
	for c := range values {
		result = append(result, c)
	}
	return result
}
func (s *ConnectionSet) CloseUser(userID int64) {
	s.closeMatching(func(c *Connection) bool { return c.UserID == userID })
}
func (s *ConnectionSet) ClosePlatformUser(platformID, userID int64) {
	s.closeMatching(func(c *Connection) bool { return c.PlatformID == platformID && c.UserID == userID })
}
func (s *ConnectionSet) ClosePlatform(platformID int64) {
	s.closeMatching(func(c *Connection) bool { return c.PlatformID == platformID })
}
func (s *ConnectionSet) CloseAll() { s.closeMatching(func(*Connection) bool { return true }) }
func (s *ConnectionSet) BeginDrain() {
	s.mu.Lock()
	s.draining = true
	targets := make([]*Connection, 0, len(s.bySession))
	for _, c := range s.bySession {
		targets = append(targets, c)
	}
	for _, c := range targets {
		s.detachLocked(c)
	}
	snapshot, observer := s.subscriptionSnapshotLocked()
	s.mu.Unlock()
	if observer != nil {
		observer(snapshot)
	}
	for _, c := range targets {
		c.Close(1001, "server draining")
	}
}
func (s *ConnectionSet) closeMatching(match func(*Connection) bool) {
	s.mu.Lock()
	var targets []*Connection
	for _, c := range s.bySession {
		if match(c) {
			targets = append(targets, c)
			s.detachLocked(c)
		}
	}
	snapshot, observer := s.subscriptionSnapshotLocked()
	s.mu.Unlock()
	if observer != nil {
		observer(snapshot)
	}
	for _, c := range targets {
		c.Close(1001, "connection closed")
	}
}
func (s *ConnectionSet) String() string { return fmt.Sprintf("connections=%d", s.Len()) }

func (s *ConnectionSet) SetSubscriptionObserver(observer func(DesiredSubscriptions)) {
	s.mu.Lock()
	s.subscriptionObserver = observer
	snapshot, current := s.subscriptionSnapshotLocked()
	s.mu.Unlock()
	if current != nil {
		current(snapshot)
	}
}

func (s *ConnectionSet) subscriptionSnapshotLocked() (DesiredSubscriptions, func(DesiredSubscriptions)) {
	snapshot := DesiredSubscriptions{PlatformCodes: make(map[string]int64)}
	for key := range s.byUser {
		snapshot.Users = append(snapshot.Users, UserSubscription{PlatformID: key.platformID, UserID: key.userID})
	}
	for platformID, connections := range s.byPlatform {
		snapshot.Platforms = append(snapshot.Platforms, platformID)
		for connection := range connections {
			if connection.PlatformCode != "" {
				snapshot.PlatformCodes[connection.PlatformCode] = platformID
			}
		}
	}
	normalized, err := normalizeDesiredSubscriptions(snapshot)
	if err != nil {
		return DesiredSubscriptions{PlatformCodes: map[string]int64{}}, s.subscriptionObserver
	}
	return normalized, s.subscriptionObserver
}
