package task

type CreateRequest struct {
	Title       string `json:"title" binding:"required,min=1,max=160"`
	Description string `json:"description" binding:"max=2000"`
}
type UpdateRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Completed   *bool   `json:"completed"`
}
