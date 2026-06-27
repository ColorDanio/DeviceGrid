package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/michael/device_grid/internal/auth"
	"github.com/michael/device_grid/internal/model"
	nodepkg "github.com/michael/device_grid/internal/node"
	"github.com/michael/device_grid/internal/store/repo"
)

type AuthHandler struct {
	repos repo.Repositories
	jm    *auth.JWTManager
}

func NewAuthHandler(repos repo.Repositories, jm *auth.JWTManager) *AuthHandler {
	return &AuthHandler{repos: repos, jm: jm}
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request: "+err.Error())
		return
	}

	user, err := h.repos.Users().GetByUsername(c.Request.Context(), req.Username)
	if err != nil {
		c.JSON(http.StatusUnauthorized, APIResponse{Code: 401, Message: "invalid credentials"})
		return
	}

	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		c.JSON(http.StatusUnauthorized, APIResponse{Code: 401, Message: "invalid credentials"})
		return
	}

	token, err := h.jm.Generate(user.ID, user.Username, user.Role)
	if err != nil {
		InternalError(c, "generate token: "+err.Error())
		return
	}

	OK(c, gin.H{
		"token":    token,
		"username": user.Username,
		"role":     user.Role,
	})
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	userID, _ := c.Get("user_id")
	username, _ := c.Get("username")
	roleStr, _ := c.Get("role")

	token, err := h.jm.Generate(userID.(string), username.(string), model.UserRole(roleStr.(string)))
	if err != nil {
		InternalError(c, "generate token: "+err.Error())
		return
	}
	OK(c, gin.H{"token": token})
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID, _ := c.Get("user_id")
	username, _ := c.Get("username")
	role, _ := c.Get("role")
	OK(c, gin.H{
		"user_id":  userID,
		"username": username,
		"role":     role,
	})
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}
	if len(req.NewPassword) < 8 {
		BadRequest(c, "password must be at least 8 characters")
		return
	}

	user, err := h.repos.Users().GetByID(c.Request.Context(), userID.(string))
	if err != nil {
		Error(c, 404, "user not found")
		return
	}
	if !auth.CheckPassword(user.PasswordHash, req.OldPassword) {
		Error(c, 401, "old password incorrect")
		return
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		InternalError(c, "hash password: "+err.Error())
		return
	}
	user.PasswordHash = hash
	_ = h.repos.Users().Create(c.Request.Context(), &model.User{
		ID:           user.ID,
		Username:     user.Username,
		PasswordHash: hash,
		Role:         user.Role,
		CreatedAt:    user.CreatedAt,
	})
	OK(c, gin.H{"changed": true})
}

func EnsureDefaultUser(repos repo.Repositories) error {
	ctx := context.Background()
	_, err := repos.Users().GetByUsername(ctx, "admin")
	if err == nil {
		return nil
	}

	hash, err := auth.HashPassword("admin123")
	if err != nil {
		return err
	}

	user := &model.User{
		ID:           uuid.NewString(),
		Username:     "admin",
		PasswordHash: hash,
		Role:         model.RoleAdmin,
		CreatedAt:    time.Now(),
	}
	if err := repos.Users().Create(ctx, user); err != nil {
		return err
	}

	// Flag the node for forced password change
	nodes, _ := repos.Nodes().List(ctx, model.NodeFilter{})
	for _, n := range nodes {
		n.ForcePasswordChange = true
		_ = repos.Nodes().Update(ctx, n)
	}
	return nil
}

// BackfillGeo populates geo info for nodes that don't have it yet.
func BackfillGeo(repos repo.Repositories, enableGeo bool) {
	if !enableGeo {
		return
	}
	ctx := context.Background()
	nodes, err := repos.Nodes().List(ctx, model.NodeFilter{})
	if err != nil {
		return
	}
	for _, n := range nodes {
		if n.CountryCode != "" {
			continue
		}
		if geo, err := nodepkg.LookupGeo(n.Host); err == nil && geo != nil {
			n.Country = geo.Country
			n.CountryCode = geo.CountryCode
			n.Region = geo.City
			n.ISP = geo.ISP
			repos.Nodes().Update(ctx, n)
		}
	}
}
