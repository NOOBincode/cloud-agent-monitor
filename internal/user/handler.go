package user

import (
	"errors"
	"time"

	"cloud-agent-monitor/internal/auth"
	"cloud-agent-monitor/internal/storage"
	"cloud-agent-monitor/internal/storage/models"
	"cloud-agent-monitor/pkg/model"
	"cloud-agent-monitor/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	userService   *Service
	apiKeyService *auth.APIKeyService
	roleRepo      storage.RoleRepositoryInterface
}

func NewHandler(userService *Service, apiKeyService *auth.APIKeyService) *Handler {
	return &Handler{
		userService:   userService,
		apiKeyService: apiKeyService,
	}
}

func NewHandlerWithRole(userService *Service, apiKeyService *auth.APIKeyService, roleRepo storage.RoleRepositoryInterface) *Handler {
	return &Handler{
		userService:   userService,
		apiKeyService: apiKeyService,
		roleRepo:      roleRepo,
	}
}

type RegisterRequest struct {
	Username    string `json:"username" binding:"required,min=3,max=50"`
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=8"`
	DisplayName string `json:"display_name" binding:"omitempty,max=100"`
}

func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequest(c, err.Error())
		return
	}

	resp, err := h.userService.Register(c.Request.Context(), RegisterRequest{
		Username:    req.Username,
		Email:       req.Email,
		Password:    req.Password,
		DisplayName: req.DisplayName,
	})
	if err != nil {
		if errors.Is(err, ErrUserExists) {
			response.ErrorWithCode(c, 409, model.CodeUserExists, "user already exists")
			return
		}
		response.InternalError(c, "failed to register user")
		return
	}

	response.Created(c, resp)
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequest(c, err.Error())
		return
	}

	ipAddress := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")

	resp, err := h.userService.Login(c.Request.Context(), LoginRequest{
		Username: req.Username,
		Password: req.Password,
	}, ipAddress, userAgent)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			response.ErrorWithCode(c, 401, model.CodeInvalidCredentials, "invalid username or password")
			return
		}
		if errors.Is(err, ErrUserInactive) {
			response.ErrorWithCode(c, 403, model.CodeUserInactive, "user account is disabled")
			return
		}
		response.InternalError(c, "login failed")
		return
	}

	response.Success(c, resp)
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (h *Handler) RefreshToken(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequest(c, err.Error())
		return
	}

	resp, err := h.userService.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.InvalidToken(c)
		return
	}

	response.Success(c, resp)
}

func (h *Handler) GetCurrentUser(c *gin.Context) {
	userID, err := h.getUserID(c)
	if err != nil {
		response.Unauthorized(c, "user not authenticated")
		return
	}

	user, err := h.userService.GetByID(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			response.NotFound(c, model.CodeUserNotFound, "user not found")
			return
		}
		response.InternalError(c, "failed to get user")
		return
	}

	response.Success(c, user)
}

type UpdateProfileRequest struct {
	DisplayName *string `json:"display_name" binding:"omitempty,max=100"`
	Email       *string `json:"email" binding:"omitempty,email"`
}

func (h *Handler) UpdateProfile(c *gin.Context) {
	userID, err := h.getUserID(c)
	if err != nil {
		response.Unauthorized(c, "user not authenticated")
		return
	}

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequest(c, err.Error())
		return
	}

	user, err := h.userService.UpdateProfile(c.Request.Context(), userID, UpdateProfileRequest{
		DisplayName: req.DisplayName,
		Email:       req.Email,
	})
	if err != nil {
		if errors.Is(err, ErrUserExists) {
			response.Conflict(c, "email already in use")
			return
		}
		response.InternalError(c, "failed to update profile")
		return
	}

	response.Success(c, user)
}

type CreateAPIKeyRequest struct {
	Name        string             `json:"name" binding:"required,min=1,max=255"`
	Permissions models.Permissions `json:"permissions"`
	ExpiresIn   *time.Duration     `json:"expires_in,omitempty"`
}

func (h *Handler) CreateAPIKey(c *gin.Context) {
	userID, err := h.getUserID(c)
	if err != nil {
		response.Unauthorized(c, "user not authenticated")
		return
	}

	var req CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequest(c, err.Error())
		return
	}

	resp, err := h.apiKeyService.CreateAPIKey(c.Request.Context(), auth.CreateAPIKeyRequest{
		UserID:      userID,
		Name:        req.Name,
		Permissions: req.Permissions,
		ExpiresIn:   req.ExpiresIn,
	})
	if err != nil {
		response.InternalError(c, "failed to create API key")
		return
	}

	response.Created(c, resp)
}

func (h *Handler) ListAPIKeys(c *gin.Context) {
	userID, err := h.getUserID(c)
	if err != nil {
		response.Unauthorized(c, "user not authenticated")
		return
	}

	keys, err := h.apiKeyService.ListUserAPIKeys(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c, "failed to list API keys")
		return
	}

	response.Success(c, keys)
}

func (h *Handler) RevokeAPIKey(c *gin.Context) {
	userID, err := h.getUserID(c)
	if err != nil {
		response.Unauthorized(c, "user not authenticated")
		return
	}

	keyIDStr := c.Param("id")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		response.ErrorWithCode(c, 400, model.CodeInvalidID, "invalid API key id format")
		return
	}

	if err := h.apiKeyService.RevokeAPIKey(c.Request.Context(), userID, keyID); err != nil {
		response.InternalError(c, "failed to revoke API key")
		return
	}

	response.NoContent(c)
}

func (h *Handler) ListUsers(c *gin.Context) {
	var filter storage.UserFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.InvalidRequest(c, err.Error())
		return
	}

	result, err := h.userService.ListUsers(c.Request.Context(), filter)
	if err != nil {
		response.InternalError(c, "failed to list users")
		return
	}

	response.Paged(c, result.Data, result.Total, result.Page, result.PageSize)
}

type SetUserStatusRequest struct {
	IsActive bool `json:"is_active"`
}

func (h *Handler) SetUserStatus(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.ErrorWithCode(c, 400, model.CodeInvalidID, "invalid user id format")
		return
	}

	var req SetUserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequest(c, err.Error())
		return
	}

	if err := h.userService.SetUserStatus(c.Request.Context(), userID, req.IsActive); err != nil {
		if errors.Is(err, ErrUserNotFound) {
			response.NotFound(c, model.CodeUserNotFound, "user not found")
			return
		}
		response.InternalError(c, "failed to update user status")
		return
	}

	response.SuccessWithMessage(c, "user status updated", nil)
}

func (h *Handler) GetUserRoles(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.ErrorWithCode(c, 400, model.CodeInvalidID, "invalid user id format")
		return
	}

	roles, err := h.userService.GetUserRoles(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c, "failed to get user roles")
		return
	}

	response.Success(c, roles)
}

type AssignRoleRequest struct {
	RoleID string `json:"role_id" binding:"required"`
}

func (h *Handler) AssignRole(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.ErrorWithCode(c, 400, model.CodeInvalidID, "invalid user id format")
		return
	}

	var req AssignRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequest(c, err.Error())
		return
	}

	roleID, err := uuid.Parse(req.RoleID)
	if err != nil {
		response.ErrorWithCode(c, 400, model.CodeInvalidID, "invalid role id format")
		return
	}

	if err := h.userService.AssignRole(c.Request.Context(), userID, roleID); err != nil {
		response.InternalError(c, "failed to assign role")
		return
	}

	response.SuccessWithMessage(c, "role assigned", nil)
}

func (h *Handler) RemoveRole(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.ErrorWithCode(c, 400, model.CodeInvalidID, "invalid user id format")
		return
	}

	roleIDStr := c.Param("role_id")
	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		response.ErrorWithCode(c, 400, model.CodeInvalidID, "invalid role id format")
		return
	}

	if err := h.userService.RemoveRole(c.Request.Context(), userID, roleID); err != nil {
		response.InternalError(c, "failed to remove role")
		return
	}

	response.SuccessWithMessage(c, "role removed", nil)
}

func (h *Handler) ListRoles(c *gin.Context) {
	var filter storage.RoleFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.InvalidRequest(c, err.Error())
		return
	}

	result, err := h.roleRepo.List(c.Request.Context(), filter)
	if err != nil {
		response.InternalError(c, "failed to list roles")
		return
	}

	response.Paged(c, result.Data, result.Total, result.Page, result.PageSize)
}

func (h *Handler) GetLoginLogs(c *gin.Context) {
	userID, err := h.getUserID(c)
	if err != nil {
		response.Unauthorized(c, "user not authenticated")
		return
	}

	logs, err := h.userService.GetLoginLogs(c.Request.Context(), userID, 20)
	if err != nil {
		response.InternalError(c, "failed to get login logs")
		return
	}

	response.Success(c, logs)
}

type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type ForgotPasswordResponse struct {
	Message string `json:"message"`
	Token   string `json:"token,omitempty"`
}

func (h *Handler) ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequest(c, err.Error())
		return
	}

	token, err := h.userService.ForgotPassword(c.Request.Context(), req.Email)
	if err != nil {
		response.Success(c, ForgotPasswordResponse{
			Message: "if the email exists, a reset link has been sent",
		})
		return
	}

	response.Success(c, ForgotPasswordResponse{
		Message: "if the email exists, a reset link has been sent",
		Token:   token,
	})
}

type ResetPasswordRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required,min=8"`
}

func (h *Handler) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequest(c, err.Error())
		return
	}

	if err := h.userService.ResetPassword(c.Request.Context(), req.Token, req.Password); err != nil {
		if errors.Is(err, ErrInvalidToken) || errors.Is(err, ErrTokenExpired) {
			response.ErrorWithCode(c, 400, model.CodeInvalidResetToken, "invalid or expired reset token")
			return
		}
		response.InternalError(c, "failed to reset password")
		return
	}

	response.SuccessWithMessage(c, "password reset successful", nil)
}

func (h *Handler) getUserID(c *gin.Context) (uuid.UUID, error) {
	userID, ok := auth.GetUserID(c)
	if !ok {
		return uuid.Nil, errors.New("user ID not found in context")
	}
	return userID, nil
}
