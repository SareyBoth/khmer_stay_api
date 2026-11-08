package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func registerSwagger(router *gin.Engine) {
	router.GET("/openapi.json", func(c *gin.Context) {
		c.JSON(http.StatusOK, openAPIDocument())
	})
	router.GET("/swagger", func(c *gin.Context) {
		c.Redirect(http.StatusTemporaryRedirect, "/swagger/index.html")
	})
	router.GET("/swagger/*any", ginSwagger.WrapHandler(
		swaggerFiles.Handler,
		ginSwagger.URL("/openapi.json"),
		ginSwagger.DocExpansion("list"),
		ginSwagger.PersistAuthorization(true),
	))
}

type apiOperation struct {
	Tag         string
	Summary     string
	OperationID string
	Public      bool
	Body        string
	BodyNeeded  bool
	Parameters  []any
	SuccessCode string
	SuccessText string
	SuccessType map[string]any
}

func operation(input apiOperation) map[string]any {
	code := input.SuccessCode
	if code == "" {
		code = "200"
	}
	text := input.SuccessText
	if text == "" {
		text = "Successful response"
	}
	schema := input.SuccessType
	if schema == nil && code != "204" {
		schema = objectSchema()
	}
	responses := map[string]any{
		code:  responseDescription(text, schema),
		"400": map[string]any{"$ref": "#/components/responses/BadRequest"},
		"401": map[string]any{"$ref": "#/components/responses/Unauthorized"},
		"403": map[string]any{"$ref": "#/components/responses/Forbidden"},
		"500": map[string]any{"$ref": "#/components/responses/ServerError"},
	}
	result := map[string]any{
		"tags":        []string{input.Tag},
		"summary":     input.Summary,
		"operationId": input.OperationID,
		"responses":   responses,
	}
	if input.Public {
		result["security"] = []any{}
	}
	if input.Body != "" {
		result["requestBody"] = map[string]any{
			"required": input.BodyNeeded,
			"content": map[string]any{
				"application/json": map[string]any{"schema": schemaRef(input.Body)},
			},
		}
	}
	if len(input.Parameters) > 0 {
		result["parameters"] = input.Parameters
	}
	return result
}

func responseDescription(description string, schema map[string]any) map[string]any {
	result := map[string]any{"description": description}
	if schema != nil {
		result["content"] = map[string]any{
			"application/json": map[string]any{"schema": schema},
		}
	}
	return result
}

func schemaRef(name string) map[string]any {
	return map[string]any{"$ref": "#/components/schemas/" + name}
}

func objectSchema() map[string]any {
	return map[string]any{"type": "object", "additionalProperties": true}
}

func pathParameter(name, description string) map[string]any {
	return map[string]any{
		"name": name, "in": "path", "required": true, "description": description,
		"schema": map[string]any{"type": "string"},
	}
}

func queryParameter(name, description, valueType string, required bool) map[string]any {
	return map[string]any{
		"name": name, "in": "query", "required": required, "description": description,
		"schema": map[string]any{"type": valueType},
	}
}

func openAPIDocument() map[string]any {
	id := pathParameter("id", "Resource identifier")
	propertyID := pathParameter("id", "Property identifier")
	bookingID := pathParameter("id", "Booking identifier")
	conversationID := pathParameter("id", "Conversation identifier")
	paths := map[string]any{
		"/health": map[string]any{
			"get": operation(apiOperation{Tag: "System", Summary: "Check API health", OperationID: "health", Public: true}),
		},
		"/events": map[string]any{
			"get": operation(apiOperation{Tag: "System", Summary: "Subscribe to authenticated server-sent events", OperationID: "events", Parameters: []any{queryParameter("token", "JWT for EventSource clients that cannot set an Authorization header", "string", false)}, SuccessType: map[string]any{"type": "string"}}),
		},
		"/ws": map[string]any{
			"get": websocketOperation(),
		},
		"/currency/rates": map[string]any{
			"get": operation(apiOperation{Tag: "System", Summary: "Get the current USD to KHR rate", OperationID: "currencyRates", Public: true, SuccessType: schemaRef("CurrencyRate")}),
		},
		"/payments/bakong/webhook": map[string]any{
			"post": operation(apiOperation{Tag: "Payments", Summary: "Receive a Bakong payment notification", OperationID: "bakongWebhook", Public: true, Body: "BakongWebhookRequest", BodyNeeded: true}),
		},
		"/auth/register": map[string]any{
			"post": operation(apiOperation{Tag: "Authentication", Summary: "Create an account and send an email verification code", OperationID: "register", Public: true, Body: "RegisterRequest", BodyNeeded: true, SuccessCode: "201", SuccessText: "Account created; verification code sent"}),
		},
		"/auth/login": map[string]any{
			"post": operation(apiOperation{Tag: "Authentication", Summary: "Sign in with email and password", OperationID: "login", Public: true, Body: "LoginRequest", BodyNeeded: true, SuccessType: schemaRef("AuthResponse")}),
		},
		"/auth/google": map[string]any{
			"post": operation(apiOperation{Tag: "Authentication", Summary: "Sign in or register with a Google ID token", OperationID: "googleLogin", Public: true, Body: "GoogleLoginRequest", BodyNeeded: true, SuccessType: schemaRef("AuthResponse")}),
		},
		"/auth/otp/request": map[string]any{
			"post": operation(apiOperation{Tag: "Authentication", Summary: "Send an email code", OperationID: "requestOTP", Public: true, Body: "OTPRequest", BodyNeeded: true, SuccessCode: "202"}),
		},
		"/auth/otp/verify": map[string]any{
			"post": operation(apiOperation{Tag: "Authentication", Summary: "Verify an email code and create a session", OperationID: "verifyOTP", Public: true, Body: "OTPVerifyRequest", BodyNeeded: true, SuccessType: schemaRef("AuthResponse")}),
		},
		"/auth/password/reset": map[string]any{
			"post": operation(apiOperation{Tag: "Authentication", Summary: "Reset a password with an email code", OperationID: "resetPassword", Public: true, Body: "PasswordResetRequest", BodyNeeded: true}),
		},
		"/auth/refresh": map[string]any{
			"post": operation(apiOperation{Tag: "Authentication", Summary: "Rotate a refresh token", OperationID: "refreshSession", Public: true, Body: "RefreshRequest", BodyNeeded: true, SuccessType: schemaRef("AuthResponse")}),
		},
		"/auth/me": map[string]any{
			"get": operation(apiOperation{Tag: "Authentication", Summary: "Get the authenticated user", OperationID: "currentUser"}),
		},
		"/auth/logout": map[string]any{
			"post": operation(apiOperation{Tag: "Authentication", Summary: "Revoke a refresh token", OperationID: "logout", Body: "RefreshRequest", SuccessCode: "204", SuccessText: "Logged out"}),
		},
		"/locations/provinces": map[string]any{
			"get": operation(apiOperation{Tag: "Locations", Summary: "List Cambodian provinces", OperationID: "listProvinces", Public: true}),
		},
		"/destinations": map[string]any{
			"get": operation(apiOperation{Tag: "Locations", Summary: "List destinations, optionally by province", OperationID: "listDestinations", Public: true, Parameters: []any{queryParameter("provinceCode", "Cambodian province code", "string", false)}}),
		},
		"/properties": map[string]any{
			"get":  operation(apiOperation{Tag: "Properties", Summary: "Search properties", OperationID: "listProperties", Parameters: propertySearchParameters()}),
			"post": operation(apiOperation{Tag: "Properties", Summary: "Create a property (owner or admin)", OperationID: "createProperty", Body: "PropertyInput", BodyNeeded: true, SuccessCode: "201"}),
		},
		"/properties/{id}": map[string]any{
			"get":   operation(apiOperation{Tag: "Properties", Summary: "Get a property", OperationID: "getProperty", Parameters: []any{propertyID}}),
			"patch": operation(apiOperation{Tag: "Properties", Summary: "Update a property (owner or admin)", OperationID: "updateProperty", Parameters: []any{propertyID}, Body: "PropertyInput", BodyNeeded: true}),
		},
		"/properties/{id}/status": map[string]any{
			"post": operation(apiOperation{Tag: "Properties", Summary: "Change property status (owner or admin)", OperationID: "updatePropertyStatus", Parameters: []any{propertyID}, Body: "PropertyStatusRequest", BodyNeeded: true}),
		},
		"/bookings": map[string]any{
			"get":  operation(apiOperation{Tag: "Bookings", Summary: "List bookings visible to the current user", OperationID: "listBookings"}),
			"post": operation(apiOperation{Tag: "Bookings", Summary: "Create a booking (traveler)", OperationID: "createBooking", Body: "BookingRequest", BodyNeeded: true, SuccessCode: "201"}),
		},
		"/bookings/{id}": map[string]any{
			"get": operation(apiOperation{Tag: "Bookings", Summary: "Get a booking", OperationID: "getBooking", Parameters: []any{bookingID}}),
		},
		"/bookings/{id}/status": map[string]any{
			"post": operation(apiOperation{Tag: "Bookings", Summary: "Change booking status (owner or admin)", OperationID: "updateBookingStatus", Parameters: []any{bookingID}, Body: "BookingStatusRequest", BodyNeeded: true}),
		},
		"/favorites": map[string]any{
			"get": operation(apiOperation{Tag: "Favorites", Summary: "List the traveler's favorite property IDs", OperationID: "listFavorites"}),
		},
		"/favorites/{id}": map[string]any{
			"post":   operation(apiOperation{Tag: "Favorites", Summary: "Add a favorite property", OperationID: "addFavorite", Parameters: []any{propertyID}}),
			"delete": operation(apiOperation{Tag: "Favorites", Summary: "Remove a favorite property", OperationID: "removeFavorite", Parameters: []any{propertyID}}),
		},
		"/recommendations": map[string]any{
			"get": operation(apiOperation{Tag: "Recommendations", Summary: "Get personalized property recommendations", OperationID: "recommendations", Parameters: []any{queryParameter("provinceCode", "Province code", "string", false), queryParameter("cityId", "City identifier", "string", false), queryParameter("limit", "Maximum results", "integer", false)}}),
		},
		"/recommendations/events": map[string]any{
			"post": operation(apiOperation{Tag: "Recommendations", Summary: "Record a recommendation behavior event", OperationID: "recordRecommendationEvent", Body: "RecommendationEventRequest", BodyNeeded: true, SuccessCode: "204"}),
		},
		"/bookings/{id}/payment": map[string]any{
			"get": operation(apiOperation{Tag: "Payments", Summary: "Get KHQR payment status", OperationID: "getPayment", Parameters: []any{bookingID}, SuccessType: schemaRef("PaymentStatus")}),
		},
		"/bookings/{id}/payment/check": map[string]any{
			"post": operation(apiOperation{Tag: "Payments", Summary: "Check payment with Bakong", OperationID: "checkPayment", Parameters: []any{bookingID}, SuccessType: schemaRef("PaymentStatus")}),
		},
		"/bookings/{id}/payment/qr.png": map[string]any{
			"get": imageOperation("Payments", "Download the booking KHQR image", "paymentQR", bookingID),
		},
		"/notifications": map[string]any{
			"get": operation(apiOperation{Tag: "Notifications", Summary: "List notifications", OperationID: "listNotifications"}),
		},
		"/notifications/{id}/read": map[string]any{
			"post": operation(apiOperation{Tag: "Notifications", Summary: "Mark a notification as read", OperationID: "readNotification", Parameters: []any{id}}),
		},
		"/conversations": map[string]any{
			"get":  operation(apiOperation{Tag: "Messaging", Summary: "List role-scoped conversations", OperationID: "listConversations"}),
			"post": operation(apiOperation{Tag: "Messaging", Summary: "Start a property or support conversation (traveler)", OperationID: "createConversation", Body: "ConversationCreateRequest", BodyNeeded: true, SuccessCode: "201"}),
		},
		"/conversations/{id}/messages": map[string]any{
			"get":  operation(apiOperation{Tag: "Messaging", Summary: "List conversation messages", OperationID: "listMessages", Parameters: []any{conversationID}}),
			"post": operation(apiOperation{Tag: "Messaging", Summary: "Send a conversation message", OperationID: "sendMessage", Parameters: []any{conversationID}, Body: "MessageRequest", BodyNeeded: true, SuccessCode: "201"}),
		},
		"/conversations/{id}/read": map[string]any{
			"post": operation(apiOperation{Tag: "Messaging", Summary: "Mark a conversation as read", OperationID: "readConversation", Parameters: []any{conversationID}, SuccessCode: "204"}),
		},
		"/conversations/{id}/status": map[string]any{
			"post": operation(apiOperation{Tag: "Messaging", Summary: "Resolve or close a support conversation (admin)", OperationID: "updateConversationStatus", Parameters: []any{conversationID}, Body: "ConversationStatusRequest", BodyNeeded: true}),
		},
		"/blocked-dates": map[string]any{
			"get":  operation(apiOperation{Tag: "Availability", Summary: "List blocked property dates", OperationID: "listBlockedDates"}),
			"post": operation(apiOperation{Tag: "Availability", Summary: "Toggle a blocked date (owner or admin)", OperationID: "toggleBlockedDate", Body: "BlockedDateRequest", BodyNeeded: true}),
		},
		"/promotions": map[string]any{
			"get":  operation(apiOperation{Tag: "Promotions", Summary: "List promotions", OperationID: "listPromotions"}),
			"post": operation(apiOperation{Tag: "Promotions", Summary: "Create a promotion (admin)", OperationID: "createPromotion", Body: "FlexibleObject", BodyNeeded: true, SuccessCode: "201"}),
		},
		"/reviews": map[string]any{
			"get":  operation(apiOperation{Tag: "Reviews", Summary: "List reviews", OperationID: "listReviews", Parameters: []any{queryParameter("propertyId", "Filter by property", "string", false)}}),
			"post": operation(apiOperation{Tag: "Reviews", Summary: "Create a review (traveler)", OperationID: "createReview", Body: "ReviewRequest", BodyNeeded: true, SuccessCode: "201"}),
		},
		"/reviews/{id}/response": map[string]any{
			"post": operation(apiOperation{Tag: "Reviews", Summary: "Respond to a review (owner or admin)", OperationID: "respondToReview", Parameters: []any{id}, Body: "ReviewResponseRequest", BodyNeeded: true}),
		},
		"/dashboard": map[string]any{
			"get": operation(apiOperation{Tag: "Dashboard", Summary: "Get role-specific dashboard data", OperationID: "dashboard"}),
		},
		"/uploads": map[string]any{
			"post": uploadOperation(),
		},
		"/payouts": map[string]any{
			"get":  operation(apiOperation{Tag: "Payouts", Summary: "List payouts", OperationID: "listPayouts", Parameters: []any{queryParameter("status", "Filter by payout status", "string", false)}}),
			"post": operation(apiOperation{Tag: "Payouts", Summary: "Request a host payout (owner or admin)", OperationID: "requestPayout", Body: "PayoutRequest", BodyNeeded: true, SuccessCode: "201"}),
		},
		"/payouts/{id}/settle": map[string]any{
			"post": operation(apiOperation{Tag: "Payouts", Summary: "Settle a payout (admin)", OperationID: "settlePayout", Parameters: []any{id}, Body: "PayoutSettlementRequest", BodyNeeded: true}),
		},
		"/users": map[string]any{
			"get": operation(apiOperation{Tag: "Admin", Summary: "List users (admin)", OperationID: "listUsers"}),
		},
		"/users/{id}/status": map[string]any{
			"post": operation(apiOperation{Tag: "Admin", Summary: "Change user status (admin)", OperationID: "updateUserStatus", Parameters: []any{id}, Body: "UserStatusRequest", BodyNeeded: true}),
		},
		"/settings": map[string]any{
			"get":   operation(apiOperation{Tag: "Admin", Summary: "Get application settings (admin)", OperationID: "getSettings"}),
			"patch": operation(apiOperation{Tag: "Admin", Summary: "Update application settings (admin)", OperationID: "updateSettings", Body: "FlexibleObject", BodyNeeded: true}),
		},
		"/tasks": map[string]any{
			"get":  operation(apiOperation{Tag: "Tasks", Summary: "List the current user's tasks", OperationID: "listTasks"}),
			"post": operation(apiOperation{Tag: "Tasks", Summary: "Create a task", OperationID: "createTask", Body: "TaskCreateRequest", BodyNeeded: true, SuccessCode: "201"}),
		},
		"/tasks/{id}": map[string]any{
			"patch":  operation(apiOperation{Tag: "Tasks", Summary: "Update a task", OperationID: "updateTask", Parameters: []any{id}, Body: "TaskUpdateRequest", BodyNeeded: true}),
			"delete": operation(apiOperation{Tag: "Tasks", Summary: "Delete a task", OperationID: "deleteTask", Parameters: []any{id}, SuccessCode: "204"}),
		},
	}

	return map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":       "KhmerStay API",
			"version":     "1.0.0",
			"description": "Interactive documentation for the KhmerStay Gin API. Use Authorize with the JWT returned by a login endpoint. Role-restricted endpoints note the required role in their summary.",
		},
		"servers": []any{map[string]any{"url": "/api/v1", "description": "Versioned API"}},
		"tags": []any{
			tag("System"), tag("Authentication"), tag("Locations"), tag("Properties"), tag("Bookings"),
			tag("Favorites"), tag("Recommendations"), tag("Payments"), tag("Notifications"), tag("Messaging"),
			tag("Availability"), tag("Promotions"), tag("Reviews"), tag("Dashboard"), tag("Payouts"), tag("Admin"), tag("Tasks"),
		},
		"security": []any{map[string]any{"bearerAuth": []any{}}},
		"paths":    paths,
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"bearerAuth": map[string]any{"type": "http", "scheme": "bearer", "bearerFormat": "JWT"},
			},
			"responses": commonResponses(),
			"schemas":   componentSchemas(),
		},
	}
}

func tag(name string) map[string]any { return map[string]any{"name": name} }

func propertySearchParameters() []any {
	return []any{
		queryParameter("provinceCode", "Province code", "string", false),
		queryParameter("cityId", "City identifier", "string", false),
		queryParameter("checkIn", "Check-in date (YYYY-MM-DD)", "string", false),
		queryParameter("checkOut", "Check-out date (YYYY-MM-DD)", "string", false),
		queryParameter("guests", "Guest count", "integer", false),
		queryParameter("minPrice", "Minimum nightly price", "number", false),
		queryParameter("maxPrice", "Maximum nightly price", "number", false),
		queryParameter("category", "Property category", "string", false),
		queryParameter("amenities", "Comma-separated amenities", "string", false),
		queryParameter("q", "Text search", "string", false),
	}
}

func imageOperation(tagName, summary, operationID string, parameter map[string]any) map[string]any {
	return map[string]any{
		"tags": []string{tagName}, "summary": summary, "operationId": operationID,
		"parameters": []any{parameter},
		"responses": map[string]any{
			"200": map[string]any{"description": "PNG image", "content": map[string]any{"image/png": map[string]any{"schema": map[string]any{"type": "string", "format": "binary"}}}},
			"401": map[string]any{"$ref": "#/components/responses/Unauthorized"},
			"500": map[string]any{"$ref": "#/components/responses/ServerError"},
		},
	}
}

func websocketOperation() map[string]any {
	return map[string]any{
		"tags": []string{"Messaging"}, "summary": "Open the authenticated real-time WebSocket", "operationId": "chatWebSocket",
		"parameters": []any{queryParameter("token", "JWT used during the WebSocket handshake", "string", true)},
		"responses": map[string]any{
			"101": map[string]any{"description": "WebSocket connection established"},
			"401": map[string]any{"$ref": "#/components/responses/Unauthorized"},
		},
	}
}

func uploadOperation() map[string]any {
	return map[string]any{
		"tags": []string{"Properties"}, "summary": "Upload a property image (owner or admin)", "operationId": "uploadImage",
		"requestBody": map[string]any{
			"required": true,
			"content": map[string]any{"multipart/form-data": map[string]any{"schema": map[string]any{
				"type": "object", "required": []string{"file"}, "properties": map[string]any{"file": map[string]any{"type": "string", "format": "binary"}},
			}}},
		},
		"responses": map[string]any{
			"201": responseDescription("File uploaded", objectSchema()),
			"400": map[string]any{"$ref": "#/components/responses/BadRequest"},
			"401": map[string]any{"$ref": "#/components/responses/Unauthorized"},
			"403": map[string]any{"$ref": "#/components/responses/Forbidden"},
		},
	}
}

func commonResponses() map[string]any {
	errorResponse := func(description string) map[string]any {
		return map[string]any{
			"description": description,
			"content":     map[string]any{"application/json": map[string]any{"schema": schemaRef("Error")}},
		}
	}
	return map[string]any{
		"BadRequest":   errorResponse("Invalid request"),
		"Unauthorized": errorResponse("Authentication required or invalid"),
		"Forbidden":    errorResponse("Insufficient permission"),
		"ServerError":  errorResponse("Internal server error"),
	}
}

func componentSchemas() map[string]any {
	stringProperty := func(format string) map[string]any {
		result := map[string]any{"type": "string"}
		if format != "" {
			result["format"] = format
		}
		return result
	}
	object := func(required []string, properties map[string]any) map[string]any {
		result := map[string]any{"type": "object", "properties": properties}
		if len(required) > 0 {
			result["required"] = required
		}
		return result
	}
	return map[string]any{
		"Error":          object([]string{"error"}, map[string]any{"error": stringProperty("")}),
		"FlexibleObject": objectSchema(),
		"User": object([]string{"id", "email", "name", "role", "status"}, map[string]any{
			"id": stringProperty("uuid"), "email": stringProperty("email"), "name": stringProperty(""),
			"role":   map[string]any{"type": "string", "enum": []string{"traveler", "owner", "admin"}},
			"status": map[string]any{"type": "string", "enum": []string{"Pending", "Active", "Suspended", "Review"}},
		}),
		"AuthResponse": object([]string{"token", "user"}, map[string]any{
			"token": stringProperty(""), "refreshToken": stringProperty(""), "user": schemaRef("User"),
		}),
		"RegisterRequest": object([]string{"email", "password", "name"}, map[string]any{
			"email": stringProperty("email"), "password": map[string]any{"type": "string", "minLength": 8, "maxLength": 72, "format": "password"},
			"name": map[string]any{"type": "string", "minLength": 2}, "role": map[string]any{"type": "string", "enum": []string{"traveler", "owner"}, "default": "traveler"},
		}),
		"LoginRequest":         object([]string{"email", "password"}, map[string]any{"email": stringProperty("email"), "password": stringProperty("password")}),
		"GoogleLoginRequest":   object([]string{"idToken"}, map[string]any{"idToken": stringProperty("")}),
		"OTPRequest":           object([]string{"email"}, map[string]any{"email": stringProperty("email"), "purpose": map[string]any{"type": "string", "enum": []string{"login", "verify_email", "reset"}, "default": "reset"}}),
		"OTPVerifyRequest":     object([]string{"email", "code", "purpose"}, map[string]any{"email": stringProperty("email"), "code": map[string]any{"type": "string", "pattern": "^[0-9]{6}$"}, "purpose": map[string]any{"type": "string", "enum": []string{"login", "verify_email"}}}),
		"PasswordResetRequest": object([]string{"email", "code", "password"}, map[string]any{"email": stringProperty("email"), "code": map[string]any{"type": "string", "pattern": "^[0-9]{6}$"}, "password": map[string]any{"type": "string", "minLength": 8, "maxLength": 72, "format": "password"}}),
		"RefreshRequest":       object([]string{"refreshToken"}, map[string]any{"refreshToken": stringProperty("")}),
		"CurrencyRate":         object([]string{"baseCurrency", "targetCurrency", "rate"}, map[string]any{"baseCurrency": stringProperty(""), "targetCurrency": stringProperty(""), "rate": map[string]any{"type": "number"}, "source": stringProperty(""), "timestamp": stringProperty("date-time")}),
		"PropertyInput": object([]string{"title", "provinceCode", "cityId"}, map[string]any{
			"title": stringProperty(""), "propertyType": stringProperty(""), "provinceCode": stringProperty(""), "cityId": stringProperty(""),
			"address": stringProperty(""), "description": stringProperty(""), "nightlyRate": map[string]any{"type": "number"},
			"currency": map[string]any{"type": "string", "enum": []string{"USD", "KHR"}}, "capacity": map[string]any{"type": "integer"},
			"amenities": map[string]any{"type": "array", "items": stringProperty("")}, "images": map[string]any{"type": "array", "items": stringProperty("uri")},
		}),
		"PropertyStatusRequest": object([]string{"status"}, map[string]any{"status": map[string]any{"type": "string", "enum": []string{"Draft", "Published", "Paused", "Review pending", "Rejected"}}}),
		"BookingRequest": object([]string{"propertyId", "checkIn", "checkOut", "guests"}, map[string]any{
			"propertyId": stringProperty(""), "roomId": stringProperty(""), "checkIn": stringProperty("date"), "checkOut": stringProperty("date"),
			"guests": map[string]any{"type": "integer", "minimum": 1}, "specialRequest": stringProperty(""),
		}),
		"BookingStatusRequest": object([]string{"status"}, map[string]any{"status": map[string]any{"type": "string", "enum": []string{"Pending", "New request", "Confirmed", "Checked in", "Completed", "Cancelled", "Declined", "No-show"}}}),
		"PaymentStatus": object([]string{"bookingId", "status", "amount", "currency"}, map[string]any{
			"bookingId": stringProperty(""), "status": stringProperty(""), "amount": map[string]any{"type": "number"}, "currency": stringProperty(""),
			"qrPayload": stringProperty(""), "qrMd5": stringProperty(""), "qrImageUrl": stringProperty("uri"), "expiresAt": stringProperty("date-time"),
		}),
		"BakongWebhookRequest": object([]string{"hash", "amount", "currency"}, map[string]any{
			"hash": stringProperty(""), "bookingId": stringProperty(""), "billNumber": stringProperty(""), "amount": map[string]any{"type": "number"},
			"currency": map[string]any{"type": "string", "enum": []string{"USD", "KHR"}}, "secretToken": stringProperty(""),
		}),
		"RecommendationEventRequest": object([]string{"propertyId", "eventType"}, map[string]any{"propertyId": stringProperty(""), "eventType": stringProperty(""), "metadata": objectSchema()}),
		"MessageRequest":             object([]string{"body"}, map[string]any{"body": map[string]any{"type": "string", "minLength": 1, "maxLength": 2000}}),
		"ConversationCreateRequest":  object([]string{"type"}, map[string]any{"type": map[string]any{"type": "string", "enum": []string{"property", "support"}}, "propertyId": stringProperty(""), "subject": map[string]any{"type": "string", "maxLength": 160}}),
		"ConversationStatusRequest":  object([]string{"status"}, map[string]any{"status": map[string]any{"type": "string", "enum": []string{"open", "resolved", "closed"}}}),
		"BlockedDateRequest":         object([]string{"propertyId", "date"}, map[string]any{"propertyId": stringProperty(""), "date": stringProperty("date")}),
		"ReviewRequest":              object([]string{"propertyId", "rating", "comment"}, map[string]any{"propertyId": stringProperty(""), "rating": map[string]any{"type": "integer", "minimum": 1, "maximum": 5}, "comment": map[string]any{"type": "string", "minLength": 2, "maxLength": 2000}}),
		"ReviewResponseRequest":      object([]string{"response"}, map[string]any{"response": map[string]any{"type": "string", "maxLength": 2000}}),
		"PayoutRequest":              object([]string{"bookingId", "bakongAccount"}, map[string]any{"bookingId": stringProperty(""), "bakongAccount": stringProperty("")}),
		"PayoutSettlementRequest":    object([]string{"reference"}, map[string]any{"reference": stringProperty(""), "status": map[string]any{"type": "string", "enum": []string{"Processing", "Paid", "Declined"}}}),
		"UserStatusRequest":          object([]string{"status"}, map[string]any{"status": map[string]any{"type": "string", "enum": []string{"Active", "Suspended", "Review"}}}),
		"TaskCreateRequest":          object([]string{"title"}, map[string]any{"title": map[string]any{"type": "string", "maxLength": 160}, "description": map[string]any{"type": "string", "maxLength": 2000}}),
		"TaskUpdateRequest":          object(nil, map[string]any{"title": stringProperty(""), "description": stringProperty(""), "completed": map[string]any{"type": "boolean"}}),
	}
}
