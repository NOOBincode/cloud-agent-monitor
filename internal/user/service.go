package user

import (
	"context"
	"errors"
	"time"

	"cloud-agent-monitor/internal/auth"
	"cloud-agent-monitor/internal/storage"
	"cloud-agent-monitor/internal/storage/models"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrUserExists         = errors.New("user already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserInactive       = errors.New("user is inactive")
	ErrInvalidToken       = errors.New("invalid reset token")
	ErrTokenExpired       = errors.New("reset token expired")
)

type Service struct {
	userRepo   storage.UserRepositoryInterface
	apiKeyRepo storage.APIKeyRepositoryInterface
	jwtService *auth.JWTService
}

func NewService(userRepo storage.UserRepositoryInterface, apiKeyRepo storage.APIKeyRepositoryInterface, jwtService *auth.JWTService) *Service {
	return &Service{
		userRepo:   userRepo,
		apiKeyRepo: apiKeyRepo,
		jwtService: jwtService,
	}
}

type UserResponse struct {
	ID          uuid.UUID `json:"id"`
	Username    string    `json:"username"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type LoginResponse struct {
	User         UserResponse `json:"user"`
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresIn    int64        `json:"expires_in"`
	TokenType    string       `json:"token_type"`
	APIKey       string       `json:"api_key,omitempty"`
	Message      string       `json:"message"`
}

type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

func (s *Service) Register(ctx context.Context, req RegisterRequest) (*UserResponse, error) {
	exists, err := s.userRepo.ExistsByUsername(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrUserExists
	}

	exists, err = s.userRepo.ExistsByEmail(ctx, req.Email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrUserExists
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	displayName := req.DisplayName
	if displayName == "" {
		displayName = req.Username
	}

	user := &models.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: string(passwordHash),
		DisplayName:  displayName,
		IsActive:     true,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	return toUserResponse(user), nil
}

func (s *Service) Login(ctx context.Context, req LoginRequest, ipAddress, userAgent string) (*LoginResponse, error) {
	user, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			s.recordLoginLog(ctx, uuid.Nil, req.Username, ipAddress, userAgent, false, "user not found")
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if !user.IsActive {
		s.recordLoginLog(ctx, user.ID, req.Username, ipAddress, userAgent, false, "user inactive")
		return nil, ErrUserInactive
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		s.recordLoginLog(ctx, user.ID, req.Username, ipAddress, userAgent, false, "invalid password")
		return nil, ErrInvalidCredentials
	}

	s.recordLoginLog(ctx, user.ID, req.Username, ipAddress, userAgent, true, "")

	var accessToken, refreshToken string
	var expiresIn int64
	tokenType := "Bearer"

	if s.jwtService != nil {
		tenantID := ""
		if user.TenantID != nil {
			tenantID = user.TenantID.String()
		}
		tokenPair, err := s.jwtService.GenerateTokenPair(user.ID, user.Username, tenantID)
		if err != nil {
			return nil, err
		}
		accessToken = tokenPair.AccessToken
		refreshToken = tokenPair.RefreshToken
		expiresIn = tokenPair.ExpiresIn
	}

	var activeKey string
	apiKeys, err := s.apiKeyRepo.ListByUserID(ctx, user.ID)
	if err == nil {
		for _, key := range apiKeys {
			if key.IsActive && (key.ExpiresAt == nil || key.ExpiresAt.After(time.Now())) {
				activeKey = key.Prefix + "..." + key.Key[len(key.Key)-4:]
				break
			}
		}
	}

	return &LoginResponse{
		User:         *toUserResponse(user),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
		TokenType:    tokenType,
		APIKey:       activeKey,
		Message:      "login successful",
	}, nil
}

func (s *Service) RefreshToken(ctx context.Context, refreshToken string) (*RefreshTokenResponse, error) {
	if s.jwtService == nil {
		return nil, errors.New("JWT service not configured")
	}

	tokenPair, err := s.jwtService.RefreshTokens(refreshToken)
	if err != nil {
		return nil, err
	}

	return &RefreshTokenResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    tokenPair.ExpiresIn,
		TokenType:    tokenPair.TokenType,
	}, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*UserResponse, error) {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return toUserResponse(user), nil
}

func (s *Service) GetByUsername(ctx context.Context, username string) (*UserResponse, error) {
	user, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return toUserResponse(user), nil
}

func (s *Service) UpdateProfile(ctx context.Context, id uuid.UUID, req UpdateProfileRequest) (*UserResponse, error) {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	if req.DisplayName != nil {
		user.DisplayName = *req.DisplayName
	}

	if req.Email != nil && *req.Email != user.Email {
		exists, err := s.userRepo.ExistsByEmail(ctx, *req.Email)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, ErrUserExists
		}
		user.Email = *req.Email
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	return toUserResponse(user), nil
}

func (s *Service) ListUsers(ctx context.Context, filter storage.UserFilter) (*storage.UserListResult, error) {
	return s.userRepo.List(ctx, filter)
}

func (s *Service) SetUserStatus(ctx context.Context, id uuid.UUID, isActive bool) error {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return err
	}
	user.IsActive = isActive
	return s.userRepo.Update(ctx, user)
}

func (s *Service) GetUserRoles(ctx context.Context, userID uuid.UUID) ([]models.Role, error) {
	return s.userRepo.GetRoles(ctx, userID)
}

func (s *Service) AssignRole(ctx context.Context, userID, roleID uuid.UUID) error {
	return s.userRepo.AddRole(ctx, userID, roleID)
}

func (s *Service) RemoveRole(ctx context.Context, userID, roleID uuid.UUID) error {
	return s.userRepo.RemoveRole(ctx, userID, roleID)
}

func (s *Service) recordLoginLog(ctx context.Context, userID uuid.UUID, username, ipAddress, userAgent string, success bool, reason string) {
	log := &models.LoginLog{
		Username:  username,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Success:   success,
		Reason:    reason,
	}
	if userID != uuid.Nil {
		log.UserID = &userID
	}

	_ = s.userRepo.CreateLoginLog(ctx, log)
}

func (s *Service) GetLoginLogs(ctx context.Context, userID uuid.UUID, limit int) ([]models.LoginLog, error) {
	return s.userRepo.GetLoginLogsByUserID(ctx, userID, limit)
}

func (s *Service) ForgotPassword(ctx context.Context, email string) (string, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return "", nil
	}

	if !user.IsActive {
		return "", nil
	}

	resetToken := uuid.New().String()
	expiresAt := time.Now().Add(1 * time.Hour)

	user.PasswordResetToken = &resetToken
	user.PasswordResetExpires = &expiresAt

	if err := s.userRepo.Update(ctx, user); err != nil {
		return "", err
	}

	return resetToken, nil
}

func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) error {
	user, err := s.userRepo.GetByPasswordResetToken(ctx, token)
	if err != nil {
		return ErrInvalidToken
	}

	if user.PasswordResetExpires == nil || user.PasswordResetExpires.Before(time.Now()) {
		return ErrTokenExpired
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.PasswordHash = string(passwordHash)
	user.PasswordResetToken = nil
	user.PasswordResetExpires = nil

	return s.userRepo.Update(ctx, user)
}

func toUserResponse(user *models.User) *UserResponse {
	return &UserResponse{
		ID:          user.ID,
		Username:    user.Username,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		IsActive:    user.IsActive,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
	}
}
