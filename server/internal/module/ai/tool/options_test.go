package tool

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
			Types []struct {
				Value int16  `json:"value"`
				Label string `json:"label"`
			} `json:"types"`
			Statuses []struct {
				Value int16  `json:"value"`
				Label string `json:"label"`
			} `json:"statuses"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data.Types) != 2 || body.Data.Types[0].Value != 1 || len(body.Data.Statuses) != 2 {
		t.Fatalf("bad options %s", w.Body.String())
	}
}
