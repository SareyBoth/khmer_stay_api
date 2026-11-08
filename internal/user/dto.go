package user

type CreateRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Name     string `json:"name" binding:"required,min=2"`
	Role     string `json:"role"`
}
type StatusRequest struct {
	Status string `json:"status" binding:"required,oneof=Active Suspended Review"`
}
