package authcontext

import "github.com/gin-gonic/gin"

type Identity struct {
	UserID, SessionID, PlatformID int64
	Platform                      string
}

const key = "auth.context.identity"

func Set(c *gin.Context, id Identity) { c.Set(key, id) }
func Get(c *gin.Context) (Identity, bool) {
	v, ok := c.Get(key)
	if !ok {
		return Identity{}, false
	}
	id, ok := v.(Identity)
	return id, ok
}
