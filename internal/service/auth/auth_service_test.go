package auth

import (
	"testing"

	"github.com/smallfire/starfire/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// mockUserRepo is a mock implementation of repository.UserRepo
type mockUserRepo struct {
	mock.Mock
}

func (m *mockUserRepo) GetByUsername(username string) (*models.User, error) {
	args := m.Called(username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *mockUserRepo) GetByID(id int) (*models.User, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *mockUserRepo) Create(user *models.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *mockUserRepo) UpdatePassword(id int, passwordHash string) error {
	args := m.Called(id, passwordHash)
	return args.Error(0)
}

func (m *mockUserRepo) UpdateLastLoginAt(id int) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *mockUserRepo) UpdateIsActive(id int, isActive bool) error {
	args := m.Called(id, isActive)
	return args.Error(0)
}

func (m *mockUserRepo) List() ([]*models.User, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.User), args.Error(1)
}

func (m *mockUserRepo) ExistsByUsername(username string) (bool, error) {
	args := m.Called(username)
	return args.Bool(0), args.Error(1)
}

func newTestService(repo *mockUserRepo) *AuthService {
	return &AuthService{
		userRepo:  repo,
		jwtSecret: "test-secret-key-for-testing",
		jwtExpire: 3600000000000, // 1 hour in nanoseconds
		logger:    zap.NewNop(),
	}
}

func TestRegister_Success(t *testing.T) {
	repo := new(mockUserRepo)
	svc := newTestService(repo)

	repo.On("ExistsByUsername", "testuser").Return(false, nil)
	repo.On("Create", mock.Anything).Return(nil)

	result, err := svc.Register("testuser", "password123", "测试用户")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Token)
	assert.Equal(t, "testuser", result.User.Username)
	assert.Equal(t, "测试用户", result.User.Nickname)
	assert.Equal(t, models.RoleUser, result.User.Role)
	assert.True(t, result.User.IsActive)
	repo.AssertExpectations(t)
}

func TestRegister_DuplicateUsername(t *testing.T) {
	repo := new(mockUserRepo)
	svc := newTestService(repo)

	repo.On("ExistsByUsername", "existing").Return(true, nil)

	_, err := svc.Register("existing", "password123", "")

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrDuplicateUsername)
	repo.AssertExpectations(t)
}

func TestRegister_Validation(t *testing.T) {
	repo := new(mockUserRepo)
	svc := newTestService(repo)

	tests := []struct {
		name     string
		username string
		password string
		nickname string
	}{
		{"用户名太短", "ab", "password123", ""},
		{"用户名太长", "a_very_long_username_that_exceeds_32_characters", "password123", ""},
		{"用户名非法字符", "user@name", "password123", ""},
		{"密码太短", "testuser", "12345", ""},
		{"昵称太长", "testuser", "password123", "这是一个非常非常非常非常非常非常非常非常非常长的昵称超过了三十二个字符"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Register(tt.username, tt.password, tt.nickname)
			assert.Error(t, err)
			assert.ErrorIs(t, err, ErrValidation)
		})
	}
}

func TestLogin_Success(t *testing.T) {
	repo := new(mockUserRepo)
	svc := newTestService(repo)

	repo.On("GetByUsername", "testuser").Return(&models.User{
		ID:           1,
		Username:     "testuser",
		PasswordHash: "$2a$10$ruAQZsPiVzm9g9Ja0dL8MugcsDrfEYTKIKELjQ7DVgNy9goRnLp3G", // password123
		IsActive:     true,
		Role:         models.RoleUser,
	}, nil)
	repo.On("UpdateLastLoginAt", 1).Return(nil)

	result, err := svc.Login("testuser", "password123")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Token)
	assert.Equal(t, "testuser", result.User.Username)
	repo.AssertExpectations(t)
}

func TestLogin_UserNotFound(t *testing.T) {
	repo := new(mockUserRepo)
	svc := newTestService(repo)

	repo.On("GetByUsername", "nonexist").Return(nil, assert.AnError)

	_, err := svc.Login("nonexist", "password123")

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidCredentials)
	repo.AssertExpectations(t)
}

func TestLogin_UserDisabled(t *testing.T) {
	repo := new(mockUserRepo)
	svc := newTestService(repo)

	repo.On("GetByUsername", "disabled").Return(&models.User{
		ID:           2,
		Username:     "disabled",
		PasswordHash: "$2a$10$ruAQZsPiVzm9g9Ja0dL8MugcsDrfEYTKIKELjQ7DVgNy9goRnLp3G",
		IsActive:     false,
	}, nil)

	_, err := svc.Login("disabled", "password123")

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrUserDisabled)
	repo.AssertExpectations(t)
}

func TestLogin_WrongPassword(t *testing.T) {
	repo := new(mockUserRepo)
	svc := newTestService(repo)

	repo.On("GetByUsername", "testuser").Return(&models.User{
		ID:           1,
		Username:     "testuser",
		PasswordHash: "$2a$10$ruAQZsPiVzm9g9Ja0dL8MugcsDrfEYTKIKELjQ7DVgNy9goRnLp3G", // password123
		IsActive:     true,
	}, nil)

	_, err := svc.Login("testuser", "wrongpassword")

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidCredentials)
	repo.AssertExpectations(t)
}

func TestChangePassword_Success(t *testing.T) {
	repo := new(mockUserRepo)
	svc := newTestService(repo)

	repo.On("GetByID", 1).Return(&models.User{
		ID:           1,
		PasswordHash: "$2a$10$ruAQZsPiVzm9g9Ja0dL8MugcsDrfEYTKIKELjQ7DVgNy9goRnLp3G", // password123
	}, nil)
	repo.On("UpdatePassword", 1, mock.Anything).Return(nil)

	err := svc.ChangePassword(1, "password123", "newpassword456")

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestChangePassword_OldPasswordMismatch(t *testing.T) {
	repo := new(mockUserRepo)
	svc := newTestService(repo)

	repo.On("GetByID", 1).Return(&models.User{
		ID:           1,
		PasswordHash: "$2a$10$ruAQZsPiVzm9g9Ja0dL8MugcsDrfEYTKIKELjQ7DVgNy9goRnLp3G", // password123
	}, nil)

	err := svc.ChangePassword(1, "wrongoldpassword", "newpassword456")

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrOldPasswordMismatch)
	repo.AssertExpectations(t)
}

func TestChangePassword_EmptyNewPassword(t *testing.T) {
	repo := new(mockUserRepo)
	svc := newTestService(repo)

	err := svc.ChangePassword(1, "password123", "")

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrValidation)
}
