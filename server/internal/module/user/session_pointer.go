package user

import "strconv"

func CurrentSessionPointerKey(userID int64) string {
	return "auth:current-session:" + strconv.FormatInt(userID, 10)
}
