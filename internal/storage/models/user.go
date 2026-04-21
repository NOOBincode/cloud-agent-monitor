package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID                   uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Username             string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"username"`
	Email                string         `gorm:"type:varchar(255);uniqueIndex" json:"email"`
	PasswordHash         string         `gorm:"type:varchar(255)" json:"-"`
	DisplayName          string         `gorm:"type:varchar(255)" json:"display_name"`
	IsActive             bool           `gorm:"default:true;index" json:"is_active"`
	TenantID             *uuid.UUID     `gorm:"type:uuid;index" json:"tenant_id,omitempty"`
	PasswordResetToken   *string        `gorm:"type:varchar(255);index" json:"-"`
	PasswordResetExpires *time.Time     `json:"-"`
	CreatedAt            time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt            time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt            gorm.DeletedAt `gorm:"index" json:"-"`

	Roles   []Role   `gorm:"many2many:user_roles;" json:"roles,omitempty"`
	APIKeys []APIKey `gorm:"foreignKey:UserID" json:"api_keys,omitempty"`
}

func (User) TableName() string {
	return "users"
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

type Role struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name        string         `gorm:"type:varchar(100);uniqueIndex;not null" json:"name"`
	Description string         `gorm:"type:text" json:"description"`
	IsSystem    bool           `gorm:"default:false;index" json:"is_system"`
	Permissions Permissions    `gorm:"type:json;serializer:json" json:"permissions"`
	TenantID    *uuid.UUID     `gorm:"type:uuid;index" json:"tenant_id,omitempty"`
	CreatedAt   time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Role) TableName() string {
	return "roles"
}

func (r *Role) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

type Permissions struct {
	ServiceRead   bool `json:"service_read"`
	ServiceWrite  bool `json:"service_write"`
	ServiceDelete bool `json:"service_delete"`
	ConfigRead    bool `json:"config_read"`
	ConfigWrite   bool `json:"config_write"`
	AuditRead     bool `json:"audit_read"`
	UserRead      bool `json:"user_read"`
	UserWrite     bool `json:"user_write"`
	Admin         bool `json:"admin"`
}

type UserRole struct {
	UserID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	RoleID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

func (UserRole) TableName() string {
	return "user_roles"
}

type APIKey struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID      uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	Name        string         `gorm:"type:varchar(255);not null" json:"name"`
	Key         string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"key"`
	KeyHash     string         `gorm:"type:varchar(255);not null" json:"-"`
	Prefix      string         `gorm:"type:varchar(20);not null" json:"prefix"`
	Permissions Permissions    `gorm:"type:json;serializer:json" json:"permissions"`
	ExpiresAt   *time.Time     `json:"expires_at,omitempty"`
	LastUsedAt  *time.Time     `json:"last_used_at,omitempty"`
	IsActive    bool           `gorm:"default:true;index" json:"is_active"`
	TenantID    *uuid.UUID     `gorm:"type:uuid;index" json:"tenant_id,omitempty"`
	CreatedAt   time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (APIKey) TableName() string {
	return "api_keys"
}

func (a *APIKey) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

type LoginLog struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID    *uuid.UUID `gorm:"type:uuid;index" json:"user_id,omitempty"`
	Username  string     `gorm:"type:varchar(255);index" json:"username"`
	IPAddress string     `gorm:"type:varchar(45)" json:"ip_address"`
	UserAgent string     `gorm:"type:varchar(500)" json:"user_agent"`
	Success   bool       `gorm:"index" json:"success"`
	Reason    string     `gorm:"type:varchar(255)" json:"reason,omitempty"`
	CreatedAt time.Time  `gorm:"autoCreateTime;index" json:"created_at"`

	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (LoginLog) TableName() string {
	return "login_logs"
}

func (l *LoginLog) BeforeCreate(tx *gorm.DB) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	return nil
}
