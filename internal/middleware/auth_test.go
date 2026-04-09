package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockUserRepoForAuth is a mock for auth middleware tests
type mockUserRepoForAuth struct {
	mock.Mock
}

func (m *mockUserRepoForAuth) GetByUsername(username string) (*models.User, error) {
	args := m.Called(username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *mockUserRepoForAuth) GetByID(id int) (*models.User, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *mockUserRepoForAuth) Create(user *models.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *mockUserRepoForAuth) UpdatePassword(id int, passwordHash string) error {
	args := m.Called(id, passwordHash)
	return args.Error(0)
}

func (m *mockUserRepoForAuth) UpdateLastLoginAt(id int) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *mockUserRepoForAuth) UpdateIsActive(id int, isActive bool) error {
	args := m.Called(id, isActive)
	return args.Error(0)
}

func (m *mockUserRepoForAuth) List() ([]*models.User, error) {
	args := m.Called()
	return args.Get(0).([]*models.User), args.Error(1)
}

func (m *mockUserRepoForAuth) ExistsByUsername(username string) (bool, error) {
	args := m.Called(username)
	return args.Bool(0), args.Error(1)
}

func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	return r
}

func generateTestToken(t *testing.T, userID int, username, role string) string {
	token, err := utils.GenerateToken(userID, username, role, "test-secret", 3600000000000)
	assert.NoError(t, err)
	return token
}

func TestAuthMiddleware_NoToken(t *testing.T) {
	r := setupRouter()
	repo := new(mockUserRepoForAuth)
	r.Use(AuthMiddleware("test-secret", repo))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	r := setupRouter()
	repo := new(mockUserRepoForAuth)
	r.Use(AuthMiddleware("test-secret", repo))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	r := setupRouter()
	repo := new(mockUserRepoForAuth)

	repo.On("GetByID", 1).Return(&models.User{
		ID:       1,
		Username: "testuser",
		IsActive: true,
		Role:     "user",
	}, nil)

	r.Use(AuthMiddleware("test-secret", repo))
	r.GET("/test", func(c *gin.Context) {
		userID, _ := GetUserID(c)
		c.JSON(http.StatusOK, gin.H{"user_id": userID})
	})

	token := generateTestToken(t, 1, "testuser", "user")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	repo.AssertExpectations(t)
}

func TestAuthMiddleware_DisabledUser(t *testing.T) {
	r := setupRouter()
	repo := new(mockUserRepoForAuth)

	repo.On("GetByID", 2).Return(&models.User{
		ID:       2,
		Username: "disabled",
		IsActive: false,
	}, nil)

	r.Use(AuthMiddleware("test-secret", repo))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	token := generateTestToken(t, 2, "disabled", "user")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	repo.AssertExpectations(t)
}

func TestAuthMiddleware_BadFormat(t *testing.T) {
	r := setupRouter()
	repo := new(mockUserRepoForAuth)
	r.Use(AuthMiddleware("test-secret", repo))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Token sometoken")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequireRole_HasPermission(t *testing.T) {
	r := setupRouter()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.Use(RequireRole("admin"))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireRole_NoPermission(t *testing.T) {
	r := setupRouter()
	r.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Next()
	})
	r.Use(RequireRole("admin"))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequireRole_NoRoleSet(t *testing.T) {
	r := setupRouter()
	r.Use(RequireRole("admin"))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}
