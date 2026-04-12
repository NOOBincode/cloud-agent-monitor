package auth

import (
	"context"
	"strings"
	"sync"
	"time"
)

type Permission string

const (
	PermissionSLORead       Permission = "slo:read"
	PermissionSLOWrite      Permission = "slo:write"
	PermissionAlertingRead  Permission = "alerting:read"
	PermissionAlertingWrite Permission = "alerting:write"
	PermissionServiceRead   Permission = "service:read"
	PermissionServiceWrite  Permission = "service:write"
	PermissionAdmin         Permission = "*"
)

type Role string

const (
	RoleViewer Role = "viewer"
	RoleEditor Role = "editor"
	RoleAdmin  Role = "admin"
	RoleAgent  Role = "agent"
)

var RolePermissions = map[Role][]Permission{
	RoleViewer: {
		PermissionSLORead,
		PermissionAlertingRead,
		PermissionServiceRead,
	},
	RoleEditor: {
		PermissionSLORead,
		PermissionSLOWrite,
		PermissionAlertingRead,
		PermissionAlertingWrite,
		PermissionServiceRead,
		PermissionServiceWrite,
	},
	RoleAdmin: {
		PermissionAdmin,
	},
	RoleAgent: {
		PermissionSLORead,
		PermissionAlertingRead,
		PermissionServiceRead,
	},
}

type UserSession struct {
	UserID      string
	Roles       []Role
	Permissions []Permission
	ExpiresAt   time.Time
}

type SessionStore interface {
	Get(ctx context.Context, token string) (*UserSession, error)
	Set(ctx context.Context, token string, session *UserSession, ttl time.Duration) error
	Delete(ctx context.Context, token string) error
}

type InMemorySessionStore struct {
	sessions map[string]*UserSession
	mu       sync.RWMutex
}

func NewInMemorySessionStore() *InMemorySessionStore {
	return &InMemorySessionStore{
		sessions: make(map[string]*UserSession),
	}
}

func (s *InMemorySessionStore) Get(ctx context.Context, token string) (*UserSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[token]
	if !ok {
		return nil, ErrSessionNotFound
	}

	if time.Now().After(session.ExpiresAt) {
		go s.Delete(context.Background(), token)
		return nil, ErrSessionExpired
	}

	return session, nil
}

func (s *InMemorySessionStore) Set(ctx context.Context, token string, session *UserSession, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session.ExpiresAt = time.Now().Add(ttl)
	s.sessions[token] = session
	return nil
}

func (s *InMemorySessionStore) Delete(ctx context.Context, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, token)
	return nil
}

var (
	ErrSessionNotFound = &AuthError{Code: "session_not_found", Message: "session not found"}
	ErrSessionExpired  = &AuthError{Code: "session_expired", Message: "session has expired"}
	ErrInvalidToken    = &AuthError{Code: "invalid_token", Message: "invalid token"}
)

type AuthError struct {
	Code    string
	Message string
}

func (e *AuthError) Error() string {
	return e.Message
}

type PermissionChecker struct {
	store SessionStore
}

func NewPermissionChecker(store SessionStore) *PermissionChecker {
	return &PermissionChecker{store: store}
}

func (c *PermissionChecker) HasPermission(ctx context.Context, userID string, permission string) (bool, error) {
	session, err := c.store.Get(ctx, userID)
	if err != nil {
		return false, err
	}

	for _, perm := range session.Permissions {
		if string(perm) == permission || string(perm) == "*" {
			return true, nil
		}
		if strings.HasSuffix(string(perm), ":*") {
			prefix := strings.TrimSuffix(string(perm), "*")
			if strings.HasPrefix(permission, prefix) {
				return true, nil
			}
		}
	}

	return false, nil
}

func (c *PermissionChecker) ValidateToken(ctx context.Context, token string) (string, error) {
	session, err := c.store.Get(ctx, token)
	if err != nil {
		return "", err
	}
	return session.UserID, nil
}

func (c *PermissionChecker) GetUserPermissions(ctx context.Context, token string) ([]string, error) {
	session, err := c.store.Get(ctx, token)
	if err != nil {
		return nil, err
	}

	perms := make([]string, len(session.Permissions))
	for i, p := range session.Permissions {
		perms[i] = string(p)
	}
	return perms, nil
}

func (c *PermissionChecker) CreateSession(userID string, roles []Role) (string, *UserSession, error) {
	token := generateToken()
	session := &UserSession{
		UserID:      userID,
		Roles:       roles,
		Permissions: getPermissionsForRoles(roles),
	}

	if err := c.store.Set(context.Background(), token, session, 24*time.Hour); err != nil {
		return "", nil, err
	}

	return token, session, nil
}

func getPermissionsForRoles(roles []Role) []Permission {
	permSet := make(map[Permission]struct{})
	for _, role := range roles {
		if perms, ok := RolePermissions[role]; ok {
			for _, perm := range perms {
				permSet[perm] = struct{}{}
			}
		}
	}

	permissions := make([]Permission, 0, len(permSet))
	for perm := range permSet {
		permissions = append(permissions, perm)
	}
	return permissions
}

func generateToken() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 32)
	for i := range b {
		b[i] = charset[i%len(charset)]
	}
	return string(b)
}
