package agent

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

func TestOptionsPreserveNumericEnumsWithoutRepository(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/", NewHandler(nil).Options)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	var body struct {
		Data struct {
			Statuses []struct {
				Value int16  `json:"value"`
				Label string `json:"label"`
			} `json:"statuses"`
			TextProtocols []struct {
				Value int16  `json:"value"`
				Label string `json:"label"`
			} `json:"textProtocols"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data.Statuses) != 2 || len(body.Data.TextProtocols) != 2 || body.Data.TextProtocols[0].Value != 1 {
		t.Fatalf("bad options %s", w.Body.String())
	}
}
