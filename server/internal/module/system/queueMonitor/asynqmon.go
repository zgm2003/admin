package queuemonitor

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"

	"admin/server/internal/queue"
	"github.com/hibiken/asynqmon"
)

//go:embed ui/dist/build/* ui/dist/build/static/js/* ui/dist/build/static/media/*
var embeddedUI embed.FS

type Monitor struct {
	official *asynqmon.HTTPHandler
	handler  http.Handler
}

func NewMonitor(redisURL string) (*Monitor, error) {
	option, err := queue.RedisConnOpt(redisURL)
	if err != nil {
		return nil, err
	}
	official := asynqmon.New(asynqmon.Options{RootPath: UIPath, RedisConnOpt: option, ReadOnly: true})
	return &Monitor{official: official, handler: newMonitorHandler(official, newUIAssetsHandler())}, nil
}

func (m *Monitor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if m == nil || m.handler == nil {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	m.handler.ServeHTTP(w, r)
}

func (m *Monitor) Close() error {
	if m == nil || m.official == nil {
		return nil
	}
	return m.official.Close()
}

func newMonitorHandler(official, assets http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiRoot := UIPath + "/api"
		if r.URL.Path == apiRoot || strings.HasPrefix(r.URL.Path, apiRoot+"/") {
			official.ServeHTTP(w, r)
			return
		}
		assets.ServeHTTP(w, r)
	})
}

type uiAssetsHandler struct{}

func newUIAssetsHandler() http.Handler { return uiAssetsHandler{} }

func (uiAssetsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL == nil || (r.URL.Path != UIPath && !strings.HasPrefix(r.URL.Path, UIPath+"/")) {
		http.Error(w, "unexpected path prefix", http.StatusBadRequest)
		return
	}
	rel := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, UIPath), "/")
	if strings.Contains(rel, "..") {
		http.Error(w, "invalid asset path", http.StatusBadRequest)
		return
	}
	if rel == "" || rel == "index.html" {
		renderIndex(w)
		return
	}
	filePath := path.Join("ui/dist/build", path.Clean("/"+rel))
	filePath = strings.ReplaceAll(filePath, "\\", "/")
	content, err := embeddedUI.ReadFile(filePath)
	if err != nil {
		if _, ok := err.(*fs.PathError); ok {
			renderIndex(w)
			return
		}
		http.Error(w, "asset unavailable", http.StatusInternalServerError)
		return
	}
	contentType := mime.TypeByExtension(path.Ext(filePath))
	if contentType == "" {
		contentType = http.DetectContentType(content)
	}
	w.Header().Set("Content-Type", contentType)
	_, _ = w.Write(content)
}

func renderIndex(w http.ResponseWriter) {
	content, err := embeddedUI.ReadFile("ui/dist/build/index.html")
	if err != nil {
		http.Error(w, "UI unavailable", http.StatusInternalServerError)
		return
	}
	localized := strings.Replace(string(content), "Asynq - Monitoring", "Asynq - 任务队列监控", 1)
	localized = strings.Replace(localized, "</head>", `<script src="/[[.RootPath]]/zh-cn.js"></script></head>`, 1)
	tmpl, err := template.New("index.html").Delims("/[[", "]]").Parse(localized)
	if err != nil {
		http.Error(w, "UI unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := struct {
		RootPath, PrometheusAddr string
		ReadOnly                 bool
	}{RootPath: UIPath, ReadOnly: true}
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, fmt.Sprintf("UI unavailable: %v", err), http.StatusInternalServerError)
	}
}
