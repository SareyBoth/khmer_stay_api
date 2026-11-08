package http

import (
	"github.com/gin-gonic/gin"
	"github.com/khmerstay/gin-api/internal/auth"
	"github.com/khmerstay/gin-api/internal/config"
	"github.com/khmerstay/gin-api/internal/currency"
	"github.com/khmerstay/gin-api/internal/events"
	"github.com/khmerstay/gin-api/internal/http/middleware"
	"github.com/khmerstay/gin-api/internal/http/response"
	"github.com/khmerstay/gin-api/internal/payment"
	"github.com/khmerstay/gin-api/internal/platform"
	"github.com/khmerstay/gin-api/internal/task"
	"github.com/khmerstay/gin-api/internal/user"
)

type Dependencies struct {
	Config    config.Config
	Tokens    *auth.TokenManager
	Auth      *auth.Handler
	Users     *user.Handler
	Tasks     *task.Handler
	Platform  *platform.Handler
	Payments  *payment.Handler
	Broker    *events.Broker
	Converter *currency.Converter
	Limiter   *middleware.RateLimiter
}

func NewRouter(deps Dependencies) *gin.Engine {
	if deps.Config.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), middleware.CORS(deps.Config.AllowedOrigins))
	registerSwagger(r)
	r.Static("/uploads", deps.Config.UploadsDir)
	register := func(api *gin.RouterGroup) {
		api.GET("/health", func(c *gin.Context) { response.JSON(c, 200, gin.H{"status": "ok"}) })
		if deps.Broker != nil {
			api.GET("/events", events.QueryAuthToken(deps.Tokens), deps.Broker.Handler())
			api.GET("/ws", deps.Broker.WebSocketHandler(deps.Tokens, deps.Config.AllowedOrigins))
		}
		if deps.Converter != nil {
			api.GET("/currency/rates", deps.Converter.Handler())
		}
		api.POST("/payments/bakong/webhook", deps.Payments.Webhook)

		// Public authentication routes with optional rate limiting
		authGroup := api.Group("")
		if deps.Limiter != nil {
			authGroup.Use(deps.Limiter.Middleware())
		}
		authGroup.POST("/auth/login", deps.Auth.Login)
		authGroup.POST("/auth/register", deps.Auth.Register)
		authGroup.POST("/auth/otp/request", deps.Auth.RequestOTP)
		authGroup.POST("/auth/otp/verify", deps.Auth.VerifyOTP)
		authGroup.POST("/auth/password/reset", deps.Auth.ResetPassword)
		authGroup.POST("/auth/refresh", deps.Auth.Refresh)
		api.POST("/auth/google", deps.Auth.Google)

		api.GET("/locations/provinces", deps.Platform.Provinces)
		api.GET("/destinations", deps.Platform.Destinations)
		protected := api.Group("")
		protected.Use(middleware.Authenticate(deps.Tokens))
		protected.GET("/auth/me", func(c *gin.Context) {
			u, _ := middleware.Current(c)
			response.JSON(c, 200, gin.H{"user": gin.H{"id": u.UserID, "email": u.Email, "name": u.Name, "role": u.Role}})
		})
		protected.POST("/auth/logout", deps.Auth.Logout)
		protected.GET("/properties", deps.Platform.ListProperties)
		protected.GET("/properties/:id", deps.Platform.GetProperty)
		protected.POST("/properties", middleware.RequireRole("owner", "admin"), deps.Platform.CreateProperty)
		protected.PATCH("/properties/:id", middleware.RequireRole("owner", "admin"), deps.Platform.UpdateProperty)
		protected.POST("/properties/:id/status", middleware.RequireRole("owner", "admin"), deps.Platform.PropertyStatus)
		protected.GET("/bookings", deps.Platform.ListBookings)
		protected.GET("/bookings/:id", deps.Platform.GetBooking)
		protected.POST("/bookings", middleware.RequireRole("traveler"), deps.Platform.CreateBooking)
		protected.POST("/bookings/:id/status", middleware.RequireRole("owner", "admin"), deps.Platform.BookingStatus)
		protected.GET("/favorites", middleware.RequireRole("traveler"), deps.Platform.Favorites)
		protected.POST("/favorites/:id", middleware.RequireRole("traveler"), deps.Platform.AddFavorite)
		protected.DELETE("/favorites/:id", middleware.RequireRole("traveler"), deps.Platform.RemoveFavorite)
		protected.GET("/recommendations", middleware.RequireRole("traveler"), deps.Platform.Recommendations)
		protected.POST("/recommendations/events", middleware.RequireRole("traveler"), deps.Platform.RecommendationEvent)
		protected.GET("/bookings/:id/payment", deps.Payments.Get)
		protected.POST("/bookings/:id/payment/check", deps.Payments.Check)
		protected.GET("/bookings/:id/payment/qr.png", deps.Payments.QR)
		protected.GET("/notifications", deps.Platform.Notifications)
		protected.POST("/notifications/:id/read", deps.Platform.ReadNotification)
		protected.GET("/conversations", deps.Platform.Conversations)
		protected.POST("/conversations", middleware.RequireRole("traveler"), deps.Platform.CreateConversation)
		protected.GET("/conversations/:id/messages", deps.Platform.Messages)
		protected.POST("/conversations/:id/messages", deps.Platform.SendMessage)
		protected.POST("/conversations/:id/read", deps.Platform.ReadConversation)
		protected.POST("/conversations/:id/status", middleware.RequireRole("admin"), deps.Platform.ConversationStatus)
		protected.GET("/blocked-dates", deps.Platform.BlockedDates)
		protected.POST("/blocked-dates", middleware.RequireRole("owner", "admin"), deps.Platform.ToggleBlockedDate)
		protected.GET("/promotions", deps.Platform.Promotions)
		protected.POST("/promotions", middleware.RequireRole("admin"), deps.Platform.CreatePromotion)
		protected.GET("/reviews", deps.Platform.Reviews)
		protected.POST("/reviews", middleware.RequireRole("traveler"), deps.Platform.CreateReview)
		protected.POST("/reviews/:id/response", middleware.RequireRole("owner", "admin"), deps.Platform.RespondReview)
		protected.GET("/dashboard", deps.Platform.Dashboard)
		protected.POST("/uploads", middleware.RequireRole("owner", "admin"), deps.Platform.Upload)

		// Host settlement & payouts
		protected.GET("/payouts", deps.Platform.ListPayouts)
		protected.POST("/payouts", middleware.RequireRole("owner", "admin"), deps.Platform.RequestPayout)
		protected.POST("/payouts/:id/settle", middleware.RequireRole("admin"), deps.Platform.SettlePayout)

		admin := protected.Group("")
		admin.Use(middleware.RequireRole("admin"))
		admin.GET("/users", deps.Users.List)
		admin.POST("/users/:id/status", deps.Users.UpdateStatus)
		admin.GET("/settings", deps.Platform.Settings)
		admin.PATCH("/settings", deps.Platform.UpdateSettings)
		protected.GET("/tasks", deps.Tasks.List)
		protected.POST("/tasks", deps.Tasks.Create)
		protected.PATCH("/tasks/:id", deps.Tasks.Update)
		protected.DELETE("/tasks/:id", deps.Tasks.Delete)
	}
	register(r.Group(""))
	register(r.Group("/api/v1"))
	return r
}
