package model

import "time"

type UserRole string

const (
	RoleAdmin    UserRole = "admin"
	RoleOperator UserRole = "operator"
	RoleViewer   UserRole = "viewer"
)

type User struct {
	ID           string    `json:"id" bson:"_id"`
	Username     string    `json:"username" bson:"username"`
	PasswordHash string    `json:"-" bson:"password_hash"`
	Role         UserRole  `json:"role" bson:"role"`
	CreatedAt    time.Time `json:"created_at" bson:"created_at"`
}
