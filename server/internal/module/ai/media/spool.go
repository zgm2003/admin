package media

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var ErrSourceInvalid = errors.New("invalid media source")

type Constraints struct {
	MaxBytes        int64
	AllowedMIMEs    []string
	SourceMIME      string
	AllowedHosts    []string
	Deadline        time.Time
	DownloadTimeout time.Duration
}
type SpoolFile struct {
	Path, MIME, SHA256 string
	SizeBytes          int64
	owner              *Spooler
	once               sync.Once
}

func (f *SpoolFile) Close() error {
	var err error
	f.once.Do(func() {
		if f.owner != nil {
			err = f.owner.release(f)
		}
	})
	return err
}

type Spooler struct {
	dir   string
	max   int64
	mu    sync.Mutex
	used  int64
	slots chan struct{}
}

func NewSpooler(directory string, maxTotalBytes int64) (*Spooler, error) {
	if maxTotalBytes <= 0 {
		return nil, errors.New("media spool quota must be positive")
	}
	if directory == "" {
		directory = os.TempDir()
	}
	d, e := os.MkdirTemp(directory, "ai-media-")
	if e != nil {
		return nil, e
	}
	if e = os.Chmod(d, 0700); e != nil {
		return nil, e
	}
	return &Spooler{dir: d, max: maxTotalBytes, slots: make(chan struct{}, 2)}, nil
}
func (s *Spooler) reserve(n int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n < 0 || n > s.max-s.used {
		return false
	}
	s.used += n
	return true
}
func (s *Spooler) release(f *SpoolFile) error {
	e := os.Remove(f.Path)
	if e != nil && !os.IsNotExist(e) {
		return e
	}
	s.mu.Lock()
	s.used -= f.SizeBytes
	if s.used < 0 {
		s.used = 0
	}
	s.mu.Unlock()
	select {
	case <-s.slots:
	default:
	}
	return nil
}
func allowedMIME(m string, allow []string) bool {
	m = normalizeMIME(m)
	switch m {
	case "image/png", "image/jpeg", "image/webp", "image/gif", "audio/mpeg", "audio/wav", "audio/ogg", "audio/mp4", "video/mp4", "video/webm":
	default:
		return false
	}
	if len(allow) == 0 {
		return false
	}
	for _, a := range allow {
		a = strings.ToLower(strings.TrimSpace(strings.Split(a, ";")[0]))
		if a == m {
			return true
		}
	}
	return false
}

func normalizeMIME(m string) string {
	m = strings.ToLower(strings.TrimSpace(strings.Split(m, ";")[0]))
	switch m {
	case "audio/wave":
		return "audio/wav"
	case "application/ogg":
		return "audio/ogg"
	default:
		return m
	}
}
func (s *Spooler) FromStream(ctx context.Context, src io.ReadCloser, declaredMIME string, declaredLength int64, c Constraints) (*SpoolFile, error) {
	if src == nil {
		return nil, errors.New("media source is required")
	}
	defer src.Close()
	readDone := make(chan struct{})
	defer close(readDone)
	go func() {
		select {
		case <-ctx.Done():
			_ = src.Close()
		case <-readDone:
		}
	}()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.MaxBytes <= 0 || declaredLength > c.MaxBytes || !allowedMIME(declaredMIME, c.AllowedMIMEs) {
		return nil, fmt.Errorf("%w: media source rejected", ErrSourceInvalid)
	}
	buffered := bufio.NewReader(src)
	prefix, _ := buffered.Peek(12)
	if len(prefix) > 0 && !headerCompatible(declaredMIME, prefix) {
		return nil, fmt.Errorf("%w: media content signature rejected", ErrSourceInvalid)
	}
	select {
	case s.slots <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	name := filepath.Join(s.dir, fmt.Sprintf("%d.bin", time.Now().UnixNano()))
	f, e := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		<-s.slots
		return nil, e
	}
	h := sha256.New()
	var n int64
	buf := make([]byte, 32<<10)
	for {
		if err := ctx.Err(); err != nil {
			f.Close()
			os.Remove(name)
			s.mu.Lock()
			s.used -= n
			s.mu.Unlock()
			<-s.slots
			return nil, err
		}
		r, e := buffered.Read(buf)
		if r > 0 {
			if n == 0 && !headerCompatible(declaredMIME, buf[:r]) {
				f.Close()
				os.Remove(name)
				<-s.slots
				return nil, fmt.Errorf("%w: media content signature rejected", ErrSourceInvalid)
			}
			if n+int64(r) > c.MaxBytes {
				f.Close()
				os.Remove(name)
				s.mu.Lock()
				s.used -= n
				s.mu.Unlock()
				<-s.slots
				return nil, fmt.Errorf("%w: media source exceeds limit", ErrSourceInvalid)
			}
			if !s.reserve(int64(r)) {
				f.Close()
				os.Remove(name)
				s.mu.Lock()
				s.used -= n
				s.mu.Unlock()
				<-s.slots
				return nil, errors.New("media spool quota exceeded")
			}
			if _, e = f.Write(buf[:r]); e != nil {
				f.Close()
				os.Remove(name)
				s.mu.Lock()
				s.used -= n + int64(r)
				s.mu.Unlock()
				<-s.slots
				return nil, e
			}
			h.Write(buf[:r])
			n += int64(r)
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			f.Close()
			os.Remove(name)
			s.mu.Lock()
			s.used -= n
			s.mu.Unlock()
			<-s.slots
			return nil, e
		}
	}
	if n == 0 || (declaredLength >= 0 && n != declaredLength) {
		f.Close()
		os.Remove(name)
		s.mu.Lock()
		s.used -= n
		s.mu.Unlock()
		<-s.slots
		return nil, fmt.Errorf("%w: media source empty or length mismatch", ErrSourceInvalid)
	}
	f.Close()
	return &SpoolFile{Path: name, MIME: normalizeMIME(declaredMIME), SHA256: fmt.Sprintf("%x", h.Sum(nil)), SizeBytes: n, owner: s}, nil
}

func headerCompatible(contentType string, b []byte) bool {
	switch normalizeMIME(contentType) {
	case "image/png":
		return len(b) >= 8 && string(b[:8]) == "\x89PNG\r\n\x1a\n"
	case "image/jpeg":
		return len(b) >= 3 && b[0] == 0xff && b[1] == 0xd8 && b[2] == 0xff
	case "image/gif":
		return len(b) >= 6 && (string(b[:6]) == "GIF87a" || string(b[:6]) == "GIF89a")
	case "image/webp":
		return len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP"
	case "audio/wav":
		return len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WAVE"
	case "audio/mpeg":
		return len(b) >= 3 && (string(b[:3]) == "ID3" || (b[0] == 0xff && b[1]&0xe0 == 0xe0))
	case "audio/ogg":
		return len(b) >= 4 && string(b[:4]) == "OggS"
	case "audio/mp4", "video/mp4":
		return len(b) >= 12 && string(b[4:8]) == "ftyp"
	case "video/webm":
		return len(b) >= 4 && b[0] == 0x1a && b[1] == 0x45 && b[2] == 0xdf && b[3] == 0xa3
	}
	return false
}

func (s *Spooler) FromBase64(ctx context.Context, src io.ReadCloser, c Constraints) (*SpoolFile, error) {
	if src == nil {
		return nil, errors.New("media source is required")
	}
	mime := c.SourceMIME
	if mime == "" && len(c.AllowedMIMEs) == 1 {
		mime = c.AllowedMIMEs[0]
	}
	decoder := base64.NewDecoder(base64.StdEncoding, src)
	return s.FromStream(ctx, &base64ReadCloser{Reader: decoder, source: src}, mime, -1, c)
}

type base64ReadCloser struct {
	io.Reader
	source io.Closer
}

func (r *base64ReadCloser) Close() error { return r.source.Close() }
func (s *Spooler) FromURL(ctx context.Context, sourceURL string, c Constraints) (*SpoolFile, error) {
	if c.DownloadTimeout <= 0 || c.MaxBytes <= 0 {
		return nil, errors.New("media download constraints are invalid")
	}
	if !c.Deadline.IsZero() {
		if deadline, ok := ctx.Deadline(); !ok || c.Deadline.Before(deadline) {
			var cancel context.CancelFunc
			ctx, cancel = context.WithDeadline(ctx, c.Deadline)
			defer cancel()
		}
	}
	u, e := url.Parse(sourceURL)
	if e != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" || u.Host == "" {
		return nil, fmt.Errorf("%w: media source URL rejected", ErrSourceInvalid)
	}
	if len(c.AllowedHosts) > 0 {
		ok := false
		for _, h := range c.AllowedHosts {
			if strings.EqualFold(strings.TrimSpace(h), u.Hostname()) {
				ok = true
			}
		}
		if !ok {
			return nil, fmt.Errorf("%w: media source host rejected", ErrSourceInvalid)
		}
	}
	ips, e := net.DefaultResolver.LookupIPAddr(ctx, u.Hostname())
	if e != nil || len(ips) == 0 {
		return nil, errors.New("media source DNS rejected")
	}
	for _, ip := range ips {
		if ip.IP.IsPrivate() || ip.IP.IsLoopback() || ip.IP.IsLinkLocalUnicast() || ip.IP.IsUnspecified() || ip.IP.IsMulticast() {
			return nil, fmt.Errorf("%w: media source address rejected", ErrSourceInvalid)
		}
	}
	// Resolve once and pin the selected public address; the transport never
	// performs a second resolver lookup, preventing DNS rebinding.
	pinned := ips[0].IP.String()
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(dctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(dctx, network, net.JoinHostPort(pinned, portOrDefault(u)))
		},
	}
	defer transport.CloseIdleConnections()
	timeout := c.DownloadTimeout
	if remaining, ok := ctx.Deadline(); ok {
		if d := time.Until(remaining); d < timeout {
			timeout = d
		}
	}
	if timeout <= 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return nil, errors.New("media download deadline exceeded")
	}
	client := &http.Client{Timeout: timeout, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if e != nil {
		return nil, e
	}
	resp, e := client.Do(req)
	if e != nil {
		return nil, errors.New("media source download failed")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != 429 {
			return nil, fmt.Errorf("%w: media source download rejected", ErrSourceInvalid)
		}
		return nil, errors.New("media source download rejected")
	}
	mime := resp.Header.Get("content-type")
	if !allowedMIME(mime, c.AllowedMIMEs) {
		resp.Body.Close()
		return nil, fmt.Errorf("%w: media content type rejected", ErrSourceInvalid)
	}
	return s.FromStream(ctx, resp.Body, mime, resp.ContentLength, c)
}
func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func portOrDefault(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	return "443"
}
