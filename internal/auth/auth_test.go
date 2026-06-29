package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/michael/device_grid/internal/model"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestManager() *JWTManager {
	return NewJWTManager("test-secret-with-enough-entropy-for-hs256", 1*time.Hour)
}

func TestHashPassword_RoundTrip(t *testing.T) {
	hash, err := HashPassword("hunter2")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !CheckPassword(hash, "hunter2") {
		t.Error("CheckPassword should return true for correct password")
	}
	if CheckPassword(hash, "wrong") {
		t.Error("CheckPassword should return false for wrong password")
	}
}

func TestHashPassword_DifferentSalts(t *testing.T) {
	h1, _ := HashPassword("same")
	h2, _ := HashPassword("same")
	if h1 == h2 {
		t.Error("bcrypt should produce different hashes for same input (random salt)")
	}
}

func TestJWT_GenerateAndParse(t *testing.T) {
	m := newTestManager()
	tok, err := m.Generate("user-123", "alice", model.RoleAdmin)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	claims, err := m.Parse(tok)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("UserID: got %s want user-123", claims.UserID)
	}
	if claims.Username != "alice" {
		t.Errorf("Username: got %s want alice", claims.Username)
	}
	if claims.Role != model.RoleAdmin {
		t.Errorf("Role: got %s want admin", claims.Role)
	}
}

func TestJWT_InvalidToken(t *testing.T) {
	m := newTestManager()
	_, err := m.Parse("not-a-real-jwt")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestJWT_WrongSecret(t *testing.T) {
	m1 := NewJWTManager("secret-one-aaaaaaaaaaaaaa", time.Hour)
	m2 := NewJWTManager("secret-two-bbbbbbbbbbbbbb", time.Hour)

	tok, _ := m1.Generate("u1", "alice", model.RoleAdmin)

	// m2 should not be able to parse a token signed by m1
	_, err := m2.Parse(tok)
	if err == nil {
		t.Error("expected error when parsing with different secret")
	}
}

func TestJWT_ExpiredToken(t *testing.T) {
	// Create a manager that has already expired
	m := NewJWTManager("test-secret-with-enough-entropy-for-hs256", -1*time.Hour)
	tok, err := m.Generate("u1", "alice", model.RoleAdmin)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	_, err = m.Parse(tok)
	if err == nil {
		t.Error("expected error for expired token")
	}
}

func TestAuthRequired_MissingHeader(t *testing.T) {
	m := newTestManager()
	r := gin.New()
	r.GET("/test", AuthRequired(m), func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d want 401", w.Code)
	}
}

func TestAuthRequired_InvalidFormat(t *testing.T) {
	m := newTestManager()
	r := gin.New()
	r.GET("/test", AuthRequired(m), func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "NotBearer abc")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d want 401", w.Code)
	}
}

func TestAuthRequired_InvalidToken(t *testing.T) {
	m := newTestManager()
	r := gin.New()
	r.GET("/test", AuthRequired(m), func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d want 401", w.Code)
	}
}

func TestAuthRequired_ValidToken(t *testing.T) {
	m := newTestManager()
	r := gin.New()
	r.GET("/test", AuthRequired(m), func(c *gin.Context) {
		uid, _ := c.Get("user_id")
		role, _ := c.Get("role")
		c.JSON(200, gin.H{"user_id": uid, "role": role})
	})

	tok, _ := m.Generate("user-99", "bob", model.RoleOperator)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d want 200; body=%s", w.Code, w.Body.String())
	}
	_ = w.Body.String()
}

func TestRoleRequired_AllowedRole(t *testing.T) {
	r := gin.New()
	r.GET("/admin", func(c *gin.Context) {
		c.Set("role", "admin")
		RoleRequired("admin")(c)
	}, func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/admin", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d want 200", w.Code)
	}
}

func TestRoleRequired_DeniedRole(t *testing.T) {
	r := gin.New()
	r.GET("/admin", func(c *gin.Context) {
		c.Set("role", "viewer")
		RoleRequired("admin", "operator")(c)
	}, func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/admin", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status: got %d want 403", w.Code)
	}
}

func TestRoleRequired_NoRoleInContext(t *testing.T) {
	r := gin.New()
	r.GET("/admin", RoleRequired("admin"), func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/admin", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status: got %d want 403", w.Code)
	}
}

func TestRoleRequired_MultipleRolesAllowed(t *testing.T) {
	for _, role := range []string{"admin", "operator", "viewer"} {
		t.Run(role, func(t *testing.T) {
			r := gin.New()
			r.GET("/test", func(c *gin.Context) {
				c.Set("role", role)
				RoleRequired("admin", "operator", "viewer")(c)
			}, func(c *gin.Context) {
				c.String(200, "ok")
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/test", nil)
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("role %s: status %d want 200", role, w.Code)
			}
		})
	}
}
