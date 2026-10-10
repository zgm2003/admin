package uploadrule

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"github.com/gin-gonic/gin"
)

const maxUpdateBodyBytes = 1 << 20

// requireStatusOnUpdate is request-level authorization only. The handler still
// performs the strict DTO/duplicate-key validation against the unchanged body.
func requireStatusOnUpdate(requireStatus gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		payload, err := io.ReadAll(io.LimitReader(c.Request.Body, maxUpdateBodyBytes+1))
		if err != nil || len(payload) > maxUpdateBodyBytes {
			response.Fail(c, apperror.InvalidRequest(fmt.Errorf("update request body is invalid or too large")))
			c.Abort()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(payload))
		var fields map[string]json.RawMessage
		if err = json.Unmarshal(payload, &fields); err != nil || fields == nil {
			response.Fail(c, apperror.InvalidRequest(fmt.Errorf("update JSON must be an object")))
			c.Abort()
			return
		}
		for key := range fields {
			// encoding/json accepts case-insensitive field names; authorization must too.
			if strings.EqualFold(key, "isEnabled") {
				requireStatus(c)
				return
			}
		}
		c.Next()
	}
}
