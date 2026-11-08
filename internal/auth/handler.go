package auth

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/khmerstay/gin-api/internal/http/response"
	"github.com/khmerstay/gin-api/internal/user"
)

type Handler struct {
	service *Service
	otp     *OTPService
	google  *GoogleService
	refresh *RefreshTokenStore
}

func (h *Handler) Register(c *gin.Context) {
	var req user.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	u, err := h.service.Register(c, req)
	if err != nil {
		if err == user.ErrEmailTaken {
			existing, findErr := h.service.users.ByEmail(c, req.Email)
			if findErr == nil && existing.Status == "Pending" {
				if sendErr := h.otp.Request(c, existing.Email, "verify_email"); sendErr != nil {
					if sendErr == ErrOTPRateLimit {
						response.JSON(c, http.StatusAccepted, gin.H{"message": "a recent verification code is already available in your email"})
						return
					}
					response.Error(c, http.StatusServiceUnavailable, "the verification code could not be delivered")
					return
				}
				response.JSON(c, http.StatusAccepted, gin.H{"message": "check your email for the verification code"})
				return
			}
		}
		response.Error(c, http.StatusConflict, err.Error())
		return
	}
	if err = h.otp.Request(c, u.Email, "verify_email"); err != nil {
		log.Printf("email verification request: %v", err)
		response.Error(c, http.StatusServiceUnavailable, "account created, but the verification code could not be delivered")
		return
	}
	response.JSON(c, http.StatusCreated, gin.H{"message": "check your email for the verification code"})
}

func NewHandler(service *Service, otp *OTPService, google *GoogleService, refresh *RefreshTokenStore) *Handler {
	return &Handler{service: service, otp: otp, google: google, refresh: refresh}
}
func (h *Handler) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "valid email and password are required")
		return
	}
	token, u, err := h.service.Login(c, req.Email, req.Password)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, err.Error())
		return
	}
	resp := gin.H{"token": token, "user": u}
	if h.refresh != nil {
		if refToken, refErr := h.refresh.Create(c, u.ID); refErr == nil {
			resp["refreshToken"] = refToken
		}
	}
	response.JSON(c, 200, resp)
}

func (h *Handler) Google(c *gin.Context) {
	var req struct {
		IDToken string `json:"idToken" binding:"required"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Error(c, http.StatusBadRequest, "Google ID token is required")
		return
	}
	token, account, err := h.google.Authenticate(c, req.IDToken)
	if err != nil {
		switch err {
		case ErrGoogleNotConfigured:
			response.Error(c, http.StatusServiceUnavailable, err.Error())
		case ErrGoogleEmailConflict:
			response.Error(c, http.StatusConflict, err.Error())
		case ErrGoogleAccountDisabled:
			response.Error(c, http.StatusForbidden, err.Error())
		default:
			response.Error(c, http.StatusUnauthorized, "Google sign-in could not be verified")
		}
		return
	}
	resp := gin.H{"token": token, "user": account}
	if h.refresh != nil {
		if refToken, refErr := h.refresh.Create(c, account.ID); refErr == nil {
			resp["refreshToken"] = refToken
		}
	}
	response.JSON(c, http.StatusOK, resp)
}

func (h *Handler) RequestOTP(c *gin.Context) {
	var req struct {
		Email   string `json:"email" binding:"required,email"`
		Purpose string `json:"purpose"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Error(c, http.StatusBadRequest, "valid email is required")
		return
	}
	req.Purpose = strings.TrimSpace(req.Purpose)
	if req.Purpose == "" {
		req.Purpose = "reset"
	}
	if req.Purpose != "reset" && req.Purpose != "login" && req.Purpose != "verify_email" {
		response.Error(c, http.StatusBadRequest, "invalid OTP purpose")
		return
	}
	if err := h.otp.Request(c, req.Email, req.Purpose); err != nil {
		log.Printf("OTP request: %v", err)
	}
	response.JSON(c, http.StatusAccepted, gin.H{"message": "If the account is eligible, a code has been sent."})
}

func (h *Handler) VerifyOTP(c *gin.Context) {
	var req struct {
		Email   string `json:"email" binding:"required,email"`
		Code    string `json:"code" binding:"required,len=6,numeric"`
		Purpose string `json:"purpose" binding:"required,oneof=login verify_email"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Error(c, http.StatusBadRequest, "valid email, purpose and six-digit code are required")
		return
	}
	if err := h.otp.Verify(c, req.Email, req.Purpose, req.Code); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid or expired code")
		return
	}
	token, u, err := h.service.TokenForEmail(c, req.Email)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "account is not active")
		return
	}
	resp := gin.H{"token": token, "user": u}
	if h.refresh != nil {
		if refToken, refErr := h.refresh.Create(c, u.ID); refErr == nil {
			resp["refreshToken"] = refToken
		}
	}
	response.JSON(c, http.StatusOK, resp)
}

func (h *Handler) ResetPassword(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Code     string `json:"code" binding:"required,len=6,numeric"`
		Password string `json:"password" binding:"required,min=8,max=72"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Error(c, http.StatusBadRequest, "valid email, six-digit code and password are required")
		return
	}
	if err := h.otp.ResetPassword(c, req.Email, req.Code, req.Password); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid or expired reset code")
		return
	}
	if h.refresh != nil {
		if u, findErr := h.service.users.ByEmail(c, req.Email); findErr == nil {
			_ = h.refresh.RevokeAllForUser(c, u.ID)
		}
	}
	response.JSON(c, http.StatusOK, gin.H{"message": "password updated"})
}

func (h *Handler) Refresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refreshToken" binding:"required"`
	}
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.RefreshToken) == "" {
		response.Error(c, http.StatusBadRequest, "valid refreshToken is required")
		return
	}
	if h.refresh == nil {
		response.Error(c, http.StatusNotImplemented, "refresh token not supported")
		return
	}
	newRefresh, userID, err := h.refresh.Rotate(c, req.RefreshToken)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}
	u, err := h.service.users.ByID(c, userID)
	if err != nil || u.Status != "Active" {
		response.Error(c, http.StatusUnauthorized, "account is not active")
		return
	}
	token, err := h.service.tokens.Sign(u.ID, u.Email, u.Name, u.Role)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to sign token")
		return
	}
	response.JSON(c, http.StatusOK, gin.H{"token": token, "refreshToken": newRefresh, "user": u})
}

func (h *Handler) Logout(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refreshToken"`
	}
	_ = c.ShouldBindJSON(&req)
	if h.refresh != nil && req.RefreshToken != "" {
		_ = h.refresh.Revoke(c, req.RefreshToken)
	}
	c.Status(http.StatusNoContent)
}
