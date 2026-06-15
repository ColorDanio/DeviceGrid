package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/michael/device_grid/internal/auth"
	"github.com/michael/device_grid/internal/model"
	"github.com/michael/device_grid/internal/store/repo"
)

type UserHandler struct {
	repos repo.Repositories
	jm    *auth.JWTManager
}

func NewUserHandler(repos repo.Repositories, jm *auth.JWTManager) *UserHandler {
	return &UserHandler{repos: repos, jm: jm}
}

type createUserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Role     string `json:"role" binding:"required"`
}

func (h *UserHandler) List(c *gin.Context) {
	users, err := h.repos.Users().List(c.Request.Context())
	if err != nil {
		InternalError(c, "list users: "+err.Error())
		return
	}
	OK(c, users)
}

func (h *UserHandler) Create(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request: "+err.Error())
		return
	}

	role := model.UserRole(req.Role)
	switch role {
	case model.RoleAdmin, model.RoleOperator, model.RoleViewer:
	default:
		BadRequest(c, "invalid role, must be admin/operator/viewer")
		return
	}

	existing, err := h.repos.Users().GetByUsername(c.Request.Context(), req.Username)
	if err == nil && existing != nil {
		BadRequest(c, "username already exists")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		InternalError(c, "hash password: "+err.Error())
		return
	}

	user := &model.User{
		ID:           uuid.NewString(),
		Username:     req.Username,
		PasswordHash: hash,
		Role:         role,
		CreatedAt:    time.Now(),
	}

	if err := h.repos.Users().Create(c.Request.Context(), user); err != nil {
		InternalError(c, "create user: "+err.Error())
		return
	}

	Created(c, user)
}

func (h *UserHandler) Delete(c *gin.Context) {
	userID := c.Param("uid")
	currentUserID, _ := c.Get("user_id")
	if userID == currentUserID.(string) {
		BadRequest(c, "cannot delete yourself")
		return
	}

	if err := h.repos.Users().Delete(c.Request.Context(), userID); err != nil {
		InternalError(c, "delete user: "+err.Error())
		return
	}
	OK(c, gin.H{"deleted": true})
}

func (h *UserHandler) ChangePassword(c *gin.Context) {
	userID := c.Param("uid")
	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "password is required")
		return
	}

	user, err := h.repos.Users().GetByID(c.Request.Context(), userID)
	if err != nil {
		NotFound(c, "user not found")
		return
	}

	hash, err := auth.HashPassword(req.Password)
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

	OK(c, gin.H{"updated": true})
	_ = http.StatusOK
}
