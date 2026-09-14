package queuemonitor

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestEmbeddedUIIsChineseAndIndependentFromWorkingDirectory(t *testing.T) {
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	temp := t.TempDir()
	if err := os.Chdir(temp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })

	handler := newUIAssetsHandler()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, UIPath+"/", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "任务队列监控") || !strings.Contains(body, "/zh-cn.js") || !strings.Contains(body, `FLAG_READ_ONLY="true"`) {
		t.Fatalf("index is not localized/read-only: %s", body)
	}
}

func TestEmbeddedUIProvidesJavaScriptAndRejectsTraversal(t *testing.T) {
	handler := newUIAssetsHandler()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, UIPath+"/zh-cn.js", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "待处理") {
		t.Fatalf("localized script status=%d", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, UIPath+"/../secret", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("traversal status=%d", recorder.Code)
	}
}

type recordingHTTPHandler struct{ called bool }

func (h *recordingHTTPHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	h.called = true
	w.WriteHeader(http.StatusNoContent)
}

func TestMonitorRoutesAPIToOfficialHandlerAndAssetsLocally(t *testing.T) {
	official := &recordingHTTPHandler{}
	monitor := newMonitorHandler(official, newUIAssetsHandler())
	recorder := httptest.NewRecorder()
	monitor.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, UIPath+"/api/queues", nil))
	if !official.called || recorder.Code != http.StatusNoContent {
		t.Fatalf("official called=%v status=%d", official.called, recorder.Code)
	}
}
