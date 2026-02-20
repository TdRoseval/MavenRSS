package models

import "time"

type UserRole string

const (
	RoleUser     UserRole = "user"
	RoleAdmin    UserRole = "admin"
	RoleTemplate UserRole = "template"
)

type User struct {
	ID                int64     `json:"id"`
	Username          string    `json:"username"`
	Email             string    `json:"email"`
	PasswordHash      string    `json:"-"` 
	Role              UserRole  `json:"role"`
	Status            string    `json:"status"` 
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	InheritedFrom     *int64    `json:"inherited_from,omitempty"`
	HasInherited      bool      `json:"has_inherited"`
}

type UserQuota struct {
	ID                int64     `json:"id"`
	UserID            int64     `json:"user_id"`
	MaxFeeds          int       `json:"max_feeds"`
	MaxArticles       int64     `json:"max_articles"`
	MaxAICallsPerDay  int       `json:"max_ai_calls_per_day"`
	MaxStorageMB      int       `json:"max_storage_mb"`
	UsedFeeds         int       `json:"used_feeds"`
	UsedArticles      int64     `json:"used_articles"`
	UsedAICallsToday  int       `json:"used_ai_calls_today"`
	UsedStorageMB     int       `json:"used_storage_mb"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type UserSession struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	RefreshToken string    `json:"refresh_token"`
	UserAgent    string    `json:"user_agent"`
	IPAddress    string    `json:"ip_address"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
}

type PendingRegistration struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	PasswordHash string `json:"-"`
	CreatedAt time.Time `json:"created_at"`
}
