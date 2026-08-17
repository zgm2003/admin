package taskdemo

type CreateRequest struct {
	Message string `json:"message" binding:"required,max=200"`
}
