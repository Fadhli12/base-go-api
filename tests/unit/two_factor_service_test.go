package unit

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockUserRepositoryForTwoFactorService mocks the UserRepository interface
type MockUserRepositoryForTwoFactorService struct {
	mock.Mock
}

func (m *MockUserRepositoryForTwoFactorService) Create(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepositoryForTwoFactorService) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepositoryForTwoFactorService) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepositoryForTwoFactorService) Update(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepositoryForTwoFactorService) SoftDelete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockUserRepositoryForTwoFactorService) FindAll(ctx context.Context) ([]domain.User, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.User), args.Error(1)
}

func (m *MockUserRepositoryForTwoFactorService) UpdateTwoFactorFields(ctx context.Context, userID uuid.UUID, fields map[string]interface{}) error {
	args := m.Called(ctx, userID, fields)
	return args.Error(0)
}

func (m *MockUserRepositoryForTwoFactorService) FindWithTwoFactorStatus(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

// MockTwoFactorRecoveryCodeRepository mocks the TwoFactorRecoveryCodeRepository interface
type MockTwoFactorRecoveryCodeRepositoryForTwoFactorService struct {
	mock.Mock
}

func (m *MockTwoFactorRecoveryCodeRepositoryForTwoFactorService) Create(ctx context.Context, code *domain.TwoFactorRecoveryCode) error {
	args := m.Called(ctx, code)
	return args.Error(0)
}

func (m *MockTwoFactorRecoveryCodeRepositoryForTwoFactorService) FindByUserAndHash(ctx context.Context, userID uuid.UUID, codeHash string) (*domain.TwoFactorRecoveryCode, error) {
	args := m.Called(ctx, userID, codeHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TwoFactorRecoveryCode), args.Error(1)
}

func (m *MockTwoFactorRecoveryCodeRepositoryForTwoFactorService) MarkUsed(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTwoFactorRecoveryCodeRepositoryForTwoFactorService) DeleteAllByUser(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockTwoFactorRecoveryCodeRepositoryForTwoFactorService) CountUnusedByUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(int64), args.Error(1)
}

// MockRefreshTokenRepository mocks the RefreshTokenRepository interface
type MockRefreshTokenRepositoryForTwoFactorService struct {
	mock.Mock
}

func (m *MockRefreshTokenRepositoryForTwoFactorService) Create(ctx context.Context, token *domain.RefreshToken) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}

func (m *MockRefreshTokenRepositoryForTwoFactorService) FindByToken(ctx context.Context, tokenHash string) (*domain.RefreshToken, error) {
	args := m.Called(ctx, tokenHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.RefreshToken), args.Error(1)
}

func (m *MockRefreshTokenRepositoryForTwoFactorService) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockRefreshTokenRepositoryForTwoFactorService) DeleteByToken(ctx context.Context, tokenHash string) error {
	args := m.Called(ctx, tokenHash)
	return args.Error(0)
}

func (m *MockRefreshTokenRepositoryForTwoFactorService) DeleteExpired(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockRefreshTokenRepositoryForTwoFactorService) Revoke(ctx context.Context, tokenHash string) error {
	args := m.Called(ctx, tokenHash)
	return args.Error(0)
}

func (m *MockRefreshTokenRepositoryForTwoFactorService) FindByID(ctx context.Context, id uuid.UUID) (*domain.RefreshToken, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.RefreshToken), args.Error(1)
}

func (m *MockRefreshTokenRepositoryForTwoFactorService) FindByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.RefreshToken, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.RefreshToken), args.Error(1)
}

func (m *MockRefreshTokenRepositoryForTwoFactorService) FindByHash(ctx context.Context, hash string) (*domain.RefreshToken, error) {
	args := m.Called(ctx, hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.RefreshToken), args.Error(1)
}

func (m *MockRefreshTokenRepositoryForTwoFactorService) MarkRevoked(ctx context.Context, tokenHash string) error {
	args := m.Called(ctx, tokenHash)
	return args.Error(0)
}

func (m *MockRefreshTokenRepositoryForTwoFactorService) RevokeAllByUser(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockRefreshTokenRepositoryForTwoFactorService) FindFamilyTokens(ctx context.Context, familyID uuid.UUID) ([]*domain.RefreshToken, error) {
	args := m.Called(ctx, familyID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.RefreshToken), args.Error(1)
}

func (m *MockRefreshTokenRepositoryForTwoFactorService) RevokeFamily(ctx context.Context, familyID uuid.UUID) error {
	args := m.Called(ctx, familyID)
	return args.Error(0)
}

func (m *MockRefreshTokenRepositoryForTwoFactorService) FindPending2FAToken(ctx context.Context, userID uuid.UUID) (bool, error) {
	args := m.Called(ctx, userID)
	return args.Bool(0), args.Error(1)
}

func (m *MockRefreshTokenRepositoryForTwoFactorService) InvalidatePendingTokens(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

// encryptionKey is a 32-byte key for testing (exactly 32 bytes)
var encryptionKey = []byte("abcdefghijklmnopqrstuvwxyz123456")

func newTestTwoFactorService(
	userRepo *MockUserRepositoryForTwoFactorService,
	recoveryCodeRepo *MockTwoFactorRecoveryCodeRepositoryForTwoFactorService,
	refreshTokenRepo *MockRefreshTokenRepositoryForTwoFactorService,
) *service.TwoFactorService {
	svc := service.NewTwoFactorService(
		userRepo,
		recoveryCodeRepo,
		refreshTokenRepo,
		nil, // tokenService - can be nil for unit tests since Verify2FALogin/UseRecoveryCode tests may not exercise token generation
		service.TwoFactorServiceConfig{
			EncryptionKey: encryptionKey,
		},
		nil, // auditSvc is nil in tests
	)
	return svc
}

// TestInitiateSetup_Success tests successful 2FA setup initiation
func TestInitiateSetup_Success(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryForTwoFactorService)
	recoveryCodeRepo := new(MockTwoFactorRecoveryCodeRepositoryForTwoFactorService)
	refreshTokenRepo := new(MockRefreshTokenRepositoryForTwoFactorService)

	userID := uuid.New()
	user := &domain.User{
		ID:               userID,
		Email:            "test@example.com",
		TwoFactorEnabled: false,
		TwoFactorStatus:  string(domain.TwoFactorStatusDisabled),
	}

	userRepo.On("FindByID", ctx, userID).Return(user, nil)
	userRepo.On("Update", ctx, mock.AnythingOfType("*domain.User")).Return(nil)

	svc := newTestTwoFactorService(userRepo, recoveryCodeRepo, refreshTokenRepo)

	result, err := svc.InitiateSetup(ctx, userID)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Secret)
	assert.NotEmpty(t, result.QRCodeURL)

	userRepo.AssertExpectations(t)
}

// TestInitiateSetup_AlreadyEnabled tests 2FA setup when already enabled
func TestInitiateSetup_AlreadyEnabled(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryForTwoFactorService)
	recoveryCodeRepo := new(MockTwoFactorRecoveryCodeRepositoryForTwoFactorService)
	refreshTokenRepo := new(MockRefreshTokenRepositoryForTwoFactorService)

	userID := uuid.New()
	user := &domain.User{
		ID:               userID,
		Email:            "test@example.com",
		TwoFactorEnabled: true,
		TwoFactorStatus:  string(domain.TwoFactorStatusEnabled),
	}

	userRepo.On("FindByID", ctx, userID).Return(user, nil)

	svc := newTestTwoFactorService(userRepo, recoveryCodeRepo, refreshTokenRepo)

	result, err := svc.InitiateSetup(ctx, userID)
	assert.Nil(t, result)
	assert.NotNil(t, err)

	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		assert.Equal(t, "TWO_FACTOR_ALREADY_ENABLED", appErr.Code)
		assert.Equal(t, 400, appErr.HTTPStatus) // Service returns 400, not 409
	}

	userRepo.AssertExpectations(t)
}

// TestVerifyAndEnable_ValidTOTP tests successful TOTP verification and 2FA enable
func TestVerifyAndEnable_ValidTOTP(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryForTwoFactorService)
	recoveryCodeRepo := new(MockTwoFactorRecoveryCodeRepositoryForTwoFactorService)
	refreshTokenRepo := new(MockRefreshTokenRepositoryForTwoFactorService)

	userID := uuid.New()

	// Generate a real TOTP secret for testing
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "GoAPIBase",
		AccountName: "test@example.com",
		Period:      30,
		SecretSize:  20,
	})
	require.NoError(t, err)

	secretBytes := []byte(key.Secret())

	// Encrypt the secret as the service does (returns encrypted bytes)
	encryptedSecret, err := service.EncryptTOTPSecret(secretBytes, encryptionKey)
	require.NoError(t, err)

	// Service stores as base64.RawURLEncoding.EncodeToString(encryptedSecret)
	encodedSecret := base64.RawURLEncoding.EncodeToString(encryptedSecret)

	user := &domain.User{
		ID:               userID,
		Email:            "test@example.com",
		TwoFactorEnabled: false,
		TwoFactorStatus:  string(domain.TwoFactorStatusPending),
		TwoFactorSecret:  encodedSecret,
	}

	userRepo.On("FindByID", ctx, userID).Return(user, nil)
	userRepo.On("Update", ctx, mock.AnythingOfType("*domain.User")).Return(nil)
	recoveryCodeRepo.On("DeleteAllByUser", ctx, userID).Return(nil)
	recoveryCodeRepo.On("Create", ctx, mock.AnythingOfType("*domain.TwoFactorRecoveryCode")).Return(nil)

	svc := newTestTwoFactorService(userRepo, recoveryCodeRepo, refreshTokenRepo)

	time.Sleep(1100 * time.Millisecond)

	totpCode, err := totp.GenerateCode(key.Secret(), time.Now())
	require.NoError(t, err)

	result, err := svc.VerifyAndEnable(ctx, userID, totpCode)
	require.NoError(t, err, "VerifyAndEnable failed: %v", err)
	require.NotNil(t, result, "VerifyAndEnable should return a result on success")
	assert.Equal(t, string(domain.TwoFactorStatusEnabled), result.Status, "status should be enabled")
	assert.Len(t, result.RecoveryCodes, 8, "should return 8 recovery codes")

	userRepo.AssertExpectations(t)
	recoveryCodeRepo.AssertExpectations(t)
}

// TestVerifyAndEnable_InvalidTOTP tests TOTP verification with invalid code
func TestVerifyAndEnable_InvalidTOTP(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryForTwoFactorService)
	recoveryCodeRepo := new(MockTwoFactorRecoveryCodeRepositoryForTwoFactorService)
	refreshTokenRepo := new(MockRefreshTokenRepositoryForTwoFactorService)

	userID := uuid.New()

	// Generate a real TOTP secret for testing
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "GoAPIBase",
		AccountName: "test@example.com",
		Period:      30,
		SecretSize:  20,
	})
	require.NoError(t, err)

	secretBytes := []byte(key.Secret())

	encryptedSecret, err := service.EncryptTOTPSecret(secretBytes, encryptionKey)
	require.NoError(t, err)

	encodedSecret := base64.RawURLEncoding.EncodeToString(encryptedSecret)

	user := &domain.User{
		ID:               userID,
		Email:            "test@example.com",
		TwoFactorEnabled: false,
		TwoFactorStatus:  string(domain.TwoFactorStatusPending),
		TwoFactorSecret:  encodedSecret,
	}

	userRepo.On("FindByID", ctx, userID).Return(user, nil)

	svc := newTestTwoFactorService(userRepo, recoveryCodeRepo, refreshTokenRepo)

	// Use an invalid TOTP code
	invalidCode := "000000"

	result, err := svc.VerifyAndEnable(ctx, userID, invalidCode)
	assert.Nil(t, result)
	assert.NotNil(t, err)

	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		assert.Equal(t, "INVALID_TOTP_CODE", appErr.Code)
		assert.Equal(t, 401, appErr.HTTPStatus)
	}

	userRepo.AssertExpectations(t)
}

// TestDisable_Success tests successful 2FA disable
func TestDisable_Success(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryForTwoFactorService)
	recoveryCodeRepo := new(MockTwoFactorRecoveryCodeRepositoryForTwoFactorService)
	refreshTokenRepo := new(MockRefreshTokenRepositoryForTwoFactorService)

	userID := uuid.New()

	// Generate a real TOTP secret
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "GoAPIBase",
		AccountName: "test@example.com",
		Period:      30,
		SecretSize:  20,
	})
	require.NoError(t, err)

	secretBytes := []byte(key.Secret())

	encryptedSecret, err := service.EncryptTOTPSecret(secretBytes, encryptionKey)
	require.NoError(t, err)

	encodedSecret := base64.RawURLEncoding.EncodeToString(encryptedSecret)

	user := &domain.User{
		ID:               userID,
		Email:            "test@example.com",
		TwoFactorEnabled: true,
		TwoFactorStatus:  string(domain.TwoFactorStatusEnabled),
		TwoFactorSecret:  encodedSecret,
	}

	userRepo.On("FindByID", ctx, userID).Return(user, nil)
	userRepo.On("Update", ctx, mock.AnythingOfType("*domain.User")).Return(nil)
	recoveryCodeRepo.On("DeleteAllByUser", ctx, userID).Return(nil)

	svc := newTestTwoFactorService(userRepo, recoveryCodeRepo, refreshTokenRepo)

	// Generate a valid TOTP code
	totpCode, err := totp.GenerateCode(key.Secret(), time.Now())
	require.NoError(t, err)

	err = svc.Disable(ctx, userID, totpCode)
	require.NoError(t, err)

	userRepo.AssertExpectations(t)
	recoveryCodeRepo.AssertExpectations(t)
}

// TestDisable_NotEnabled tests disabling 2FA when it's not enabled
func TestDisable_NotEnabled(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryForTwoFactorService)
	recoveryCodeRepo := new(MockTwoFactorRecoveryCodeRepositoryForTwoFactorService)
	refreshTokenRepo := new(MockRefreshTokenRepositoryForTwoFactorService)

	userID := uuid.New()
	user := &domain.User{
		ID:               userID,
		Email:            "test@example.com",
		TwoFactorEnabled: false,
		TwoFactorStatus:  string(domain.TwoFactorStatusDisabled),
	}

	userRepo.On("FindByID", ctx, userID).Return(user, nil)

	svc := newTestTwoFactorService(userRepo, recoveryCodeRepo, refreshTokenRepo)

	err := svc.Disable(ctx, userID, "123456")
	assert.NotNil(t, err)

	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		assert.Equal(t, "2FA_NOT_ENABLED", appErr.Code)
		assert.Equal(t, 400, appErr.HTTPStatus)
	}

	userRepo.AssertExpectations(t)
}

// TestGetStatus_Enabled tests getting 2FA status when enabled
func TestGetStatus_Enabled(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryForTwoFactorService)
	recoveryCodeRepo := new(MockTwoFactorRecoveryCodeRepositoryForTwoFactorService)
	refreshTokenRepo := new(MockRefreshTokenRepositoryForTwoFactorService)

	userID := uuid.New()
	user := &domain.User{
		ID:               userID,
		Email:            "test@example.com",
		TwoFactorEnabled: true,
		TwoFactorStatus:  string(domain.TwoFactorStatusEnabled),
	}

	userRepo.On("FindByID", ctx, userID).Return(user, nil)

	svc := newTestTwoFactorService(userRepo, recoveryCodeRepo, refreshTokenRepo)

	result, err := svc.GetStatus(ctx, userID)
	require.NoError(t, err)
	assert.NotNil(t, result)

	userRepo.AssertExpectations(t)
}

// TestGetStatus_Disabled tests getting 2FA status when disabled
func TestGetStatus_Disabled(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryForTwoFactorService)
	recoveryCodeRepo := new(MockTwoFactorRecoveryCodeRepositoryForTwoFactorService)
	refreshTokenRepo := new(MockRefreshTokenRepositoryForTwoFactorService)

	userID := uuid.New()
	user := &domain.User{
		ID:               userID,
		Email:            "test@example.com",
		TwoFactorEnabled: false,
		TwoFactorStatus:  string(domain.TwoFactorStatusDisabled),
	}

	userRepo.On("FindByID", ctx, userID).Return(user, nil)

	svc := newTestTwoFactorService(userRepo, recoveryCodeRepo, refreshTokenRepo)

	result, err := svc.GetStatus(ctx, userID)
	require.NoError(t, err)
	assert.NotNil(t, result)

	userRepo.AssertExpectations(t)
}

// TestInitiateSetup_Pending tests 2FA setup when setup is already pending
func TestInitiateSetup_Pending(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryForTwoFactorService)
	recoveryCodeRepo := new(MockTwoFactorRecoveryCodeRepositoryForTwoFactorService)
	refreshTokenRepo := new(MockRefreshTokenRepositoryForTwoFactorService)

	userID := uuid.New()
	user := &domain.User{
		ID:               userID,
		Email:            "test@example.com",
		TwoFactorEnabled: false,
		TwoFactorStatus:  string(domain.TwoFactorStatusPending),
	}

	userRepo.On("FindByID", ctx, userID).Return(user, nil)

	svc := newTestTwoFactorService(userRepo, recoveryCodeRepo, refreshTokenRepo)

	result, err := svc.InitiateSetup(ctx, userID)
	assert.Nil(t, result)
	assert.NotNil(t, err)

	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		assert.Equal(t, "TWO_FACTOR_SETUP_PENDING", appErr.Code)
	}

	userRepo.AssertExpectations(t)
}

// TestVerifyAndEnable_NotPending tests verification when no setup is pending
func TestVerifyAndEnable_NotPending(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryForTwoFactorService)
	recoveryCodeRepo := new(MockTwoFactorRecoveryCodeRepositoryForTwoFactorService)
	refreshTokenRepo := new(MockRefreshTokenRepositoryForTwoFactorService)

	userID := uuid.New()
	user := &domain.User{
		ID:               userID,
		Email:            "test@example.com",
		TwoFactorEnabled: false,
		TwoFactorStatus:  string(domain.TwoFactorStatusDisabled),
	}

	userRepo.On("FindByID", ctx, userID).Return(user, nil)

	svc := newTestTwoFactorService(userRepo, recoveryCodeRepo, refreshTokenRepo)

	result, err := svc.VerifyAndEnable(ctx, userID, "123456")
	assert.Nil(t, result)
	assert.NotNil(t, err)

	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		assert.Equal(t, "TWO_FACTOR_SETUP_NOT_PENDING", appErr.Code)
	}

	userRepo.AssertExpectations(t)
}

// TestDisable_InvalidTOTP tests disabling with invalid TOTP code
func TestDisable_InvalidTOTP(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryForTwoFactorService)
	recoveryCodeRepo := new(MockTwoFactorRecoveryCodeRepositoryForTwoFactorService)
	refreshTokenRepo := new(MockRefreshTokenRepositoryForTwoFactorService)

	userID := uuid.New()

	// Generate a real TOTP secret
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "GoAPIBase",
		AccountName: "test@example.com",
		Period:      30,
		SecretSize:  20,
	})
	require.NoError(t, err)

	secretBytes := []byte(key.Secret())

	encryptedSecret, err := service.EncryptTOTPSecret(secretBytes, encryptionKey)
	require.NoError(t, err)

	encodedSecret := base64.RawURLEncoding.EncodeToString(encryptedSecret)

	user := &domain.User{
		ID:               userID,
		Email:            "test@example.com",
		TwoFactorEnabled: true,
		TwoFactorStatus:  string(domain.TwoFactorStatusEnabled),
		TwoFactorSecret:  encodedSecret,
	}

	userRepo.On("FindByID", ctx, userID).Return(user, nil)

	svc := newTestTwoFactorService(userRepo, recoveryCodeRepo, refreshTokenRepo)

	err = svc.Disable(ctx, userID, "000000")
	assert.NotNil(t, err)

	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		assert.Equal(t, "INVALID_TOTP_CODE", appErr.Code)
		assert.Equal(t, 401, appErr.HTTPStatus)
	}

	userRepo.AssertExpectations(t)
}

// TestGetStatus_Pending tests getting 2FA status when setup is pending
func TestGetStatus_Pending(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryForTwoFactorService)
	recoveryCodeRepo := new(MockTwoFactorRecoveryCodeRepositoryForTwoFactorService)
	refreshTokenRepo := new(MockRefreshTokenRepositoryForTwoFactorService)

	userID := uuid.New()

	// Generate a real TOTP secret
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "GoAPIBase",
		AccountName: "test@example.com",
		Period:      30,
		SecretSize:  20,
	})
	require.NoError(t, err)

	secretBytes := []byte(key.Secret())

	encryptedSecret, err := service.EncryptTOTPSecret(secretBytes, encryptionKey)
	require.NoError(t, err)

	user := &domain.User{
		ID:               userID,
		Email:            "test@example.com",
		TwoFactorEnabled: false,
		TwoFactorStatus:  string(domain.TwoFactorStatusPending),
		TwoFactorSecret:  string(encryptedSecret),
	}

	userRepo.On("FindByID", ctx, userID).Return(user, nil)

	svc := newTestTwoFactorService(userRepo, recoveryCodeRepo, refreshTokenRepo)

	result, err := svc.GetStatus(ctx, userID)
	require.NoError(t, err)
	assert.NotNil(t, result)

	userRepo.AssertExpectations(t)
}

// TestInitiateSetup_UserNotFound tests 2FA setup for non-existent user
func TestInitiateSetup_UserNotFound(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryForTwoFactorService)
	recoveryCodeRepo := new(MockTwoFactorRecoveryCodeRepositoryForTwoFactorService)
	refreshTokenRepo := new(MockRefreshTokenRepositoryForTwoFactorService)

	userID := uuid.New()

	userRepo.On("FindByID", ctx, userID).Return(nil, apperrors.ErrNotFound)

	svc := newTestTwoFactorService(userRepo, recoveryCodeRepo, refreshTokenRepo)

	result, err := svc.InitiateSetup(ctx, userID)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, apperrors.ErrNotFound))

	userRepo.AssertExpectations(t)
}

// TestGetStatus_UserNotFound tests getting 2FA status for non-existent user
func TestGetStatus_UserNotFound(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryForTwoFactorService)
	recoveryCodeRepo := new(MockTwoFactorRecoveryCodeRepositoryForTwoFactorService)
	refreshTokenRepo := new(MockRefreshTokenRepositoryForTwoFactorService)

	userID := uuid.New()

	userRepo.On("FindByID", ctx, userID).Return(nil, apperrors.ErrNotFound)

	svc := newTestTwoFactorService(userRepo, recoveryCodeRepo, refreshTokenRepo)

	result, err := svc.GetStatus(ctx, userID)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, apperrors.ErrNotFound))

	userRepo.AssertExpectations(t)
}

// TestDisable_UserNotFound tests disabling 2FA for non-existent user
func TestDisable_UserNotFound(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryForTwoFactorService)
	recoveryCodeRepo := new(MockTwoFactorRecoveryCodeRepositoryForTwoFactorService)
	refreshTokenRepo := new(MockRefreshTokenRepositoryForTwoFactorService)

	userID := uuid.New()

	userRepo.On("FindByID", ctx, userID).Return(nil, apperrors.ErrNotFound)

	svc := newTestTwoFactorService(userRepo, recoveryCodeRepo, refreshTokenRepo)

	err := svc.Disable(ctx, userID, "123456")
	assert.True(t, errors.Is(err, apperrors.ErrNotFound))

	userRepo.AssertExpectations(t)
}

// TestVerifyAndEnable_UserNotFound tests verification for non-existent user
func TestVerifyAndEnable_UserNotFound(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryForTwoFactorService)
	recoveryCodeRepo := new(MockTwoFactorRecoveryCodeRepositoryForTwoFactorService)
	refreshTokenRepo := new(MockRefreshTokenRepositoryForTwoFactorService)

	userID := uuid.New()

	userRepo.On("FindByID", ctx, userID).Return(nil, apperrors.ErrNotFound)

	svc := newTestTwoFactorService(userRepo, recoveryCodeRepo, refreshTokenRepo)

	result, err := svc.VerifyAndEnable(ctx, userID, "123456")
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, apperrors.ErrNotFound))

	userRepo.AssertExpectations(t)
}

// TestVerifyAndEnable_RecoveryCodeCreationFailure tests when recovery code creation fails
func TestVerifyAndEnable_RecoveryCodeCreationFailure(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryForTwoFactorService)
	recoveryCodeRepo := new(MockTwoFactorRecoveryCodeRepositoryForTwoFactorService)
	refreshTokenRepo := new(MockRefreshTokenRepositoryForTwoFactorService)

	userID := uuid.New()

	// Generate a real TOTP secret
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "GoAPIBase",
		AccountName: "test@example.com",
		Period:      30,
		SecretSize:  20,
	})
	require.NoError(t, err)

	secretBytes := []byte(key.Secret())

	encryptedSecret, err := service.EncryptTOTPSecret(secretBytes, encryptionKey)
	require.NoError(t, err)

	encodedSecret := base64.RawURLEncoding.EncodeToString(encryptedSecret)

	user := &domain.User{
		ID:               userID,
		Email:            "test@example.com",
		TwoFactorEnabled: false,
		TwoFactorStatus:  string(domain.TwoFactorStatusPending),
		TwoFactorSecret:  encodedSecret,
	}

	userRepo.On("FindByID", ctx, userID).Return(user, nil)
	userRepo.On("Update", ctx, mock.AnythingOfType("*domain.User")).Return(nil)
	recoveryCodeRepo.On("DeleteAllByUser", ctx, userID).Return(errors.New("database error"))

	svc := newTestTwoFactorService(userRepo, recoveryCodeRepo, refreshTokenRepo)

	totpCode, err := totp.GenerateCode(key.Secret(), time.Now())
	require.NoError(t, err)

	result, err := svc.VerifyAndEnable(ctx, userID, totpCode)
	assert.Nil(t, result)
	assert.NotNil(t, err)

	userRepo.AssertExpectations(t)
	recoveryCodeRepo.AssertExpectations(t)
}

// TestDisable_RecoveryCodeDeletionFailure tests when recovery code deletion fails
func TestDisable_RecoveryCodeDeletionFailure(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryForTwoFactorService)
	recoveryCodeRepo := new(MockTwoFactorRecoveryCodeRepositoryForTwoFactorService)
	refreshTokenRepo := new(MockRefreshTokenRepositoryForTwoFactorService)

	userID := uuid.New()

	// Generate a real TOTP secret
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "GoAPIBase",
		AccountName: "test@example.com",
		Period:      30,
		SecretSize:  20,
	})
	require.NoError(t, err)

	secretBytes := []byte(key.Secret())

	encryptedSecret, err := service.EncryptTOTPSecret(secretBytes, encryptionKey)
	require.NoError(t, err)

	encodedSecret := base64.RawURLEncoding.EncodeToString(encryptedSecret)

	user := &domain.User{
		ID:               userID,
		Email:            "test@example.com",
		TwoFactorEnabled: true,
		TwoFactorStatus:  string(domain.TwoFactorStatusEnabled),
		TwoFactorSecret:  encodedSecret,
	}

	userRepo.On("FindByID", ctx, userID).Return(user, nil)
	recoveryCodeRepo.On("DeleteAllByUser", ctx, userID).Return(errors.New("database error"))
	userRepo.On("Update", ctx, mock.AnythingOfType("*domain.User")).Return(nil)

	svc := newTestTwoFactorService(userRepo, recoveryCodeRepo, refreshTokenRepo)

	totpCode, err := totp.GenerateCode(key.Secret(), time.Now())
	require.NoError(t, err)

	err = svc.Disable(ctx, userID, totpCode)
	assert.NotNil(t, err)

	userRepo.AssertExpectations(t)
	recoveryCodeRepo.AssertExpectations(t)
}
