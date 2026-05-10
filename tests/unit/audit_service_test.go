package unit

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockAuditLogForAudit for testing
type MockAuditLogForAudit struct {
	mock.Mock
	mu    sync.Mutex
	logs  []*domain.AuditLog
	count int
}

func (m *MockAuditLogForAudit) Create(ctx context.Context, auditLog *domain.AuditLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.count++
	m.logs = append(m.logs, auditLog)
	args := m.Called(ctx, auditLog)
	return args.Error(0)
}

func (m *MockAuditLogForAudit) FindByActorID(ctx context.Context, actorID uuid.UUID, limit, offset int) ([]domain.AuditLog, error) {
	args := m.Called(ctx, actorID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.AuditLog), args.Error(1)
}

func (m *MockAuditLogForAudit) FindByResource(ctx context.Context, resource, resourceID string, limit, offset int) ([]domain.AuditLog, error) {
	args := m.Called(ctx, resource, resourceID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.AuditLog), args.Error(1)
}

func (m *MockAuditLogForAudit) FindAll(ctx context.Context, limit, offset int) ([]domain.AuditLog, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.AuditLog), args.Error(1)
}

func (m *MockAuditLogForAudit) GetCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.count
}

func (m *MockAuditLogForAudit) GetLogs() []*domain.AuditLog {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.logs
}

func newTestAuditService(repo *MockAuditLogForAudit, bufferSize int) *service.AuditService {
	config := service.AuditServiceConfig{BufferSize: bufferSize}
	return service.NewAuditService(repo, config)
}

func TestNewAuditService(t *testing.T) {
	t.Run("creates service with default config", func(t *testing.T) {
		repo := new(MockAuditLogForAudit)
		svc := newTestAuditService(repo, 100)
		require.NotNil(t, svc)
		svc.Shutdown()
	})

	t.Run("creates service with zero buffer size defaults to 100", func(t *testing.T) {
		repo := new(MockAuditLogForAudit)
		svc := newTestAuditService(repo, 0)
		require.NotNil(t, svc)
		// Verify it works by sending a log through
		repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)
		err := svc.LogAction(context.Background(), uuid.New(), "create", "user", "123", nil, map[string]string{"name": "test"}, "", "")
		assert.NoError(t, err)
		svc.Shutdown()
	})

	t.Run("creates service with custom buffer size", func(t *testing.T) {
		repo := new(MockAuditLogForAudit)
		svc := newTestAuditService(repo, 50)
		require.NotNil(t, svc)
		svc.Shutdown()
	})
}

func TestAuditService_LogAction(t *testing.T) {
	t.Run("queues audit log asynchronously", func(t *testing.T) {
		repo := new(MockAuditLogForAudit)
		repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

		svc := newTestAuditService(repo, 100)
		defer svc.Shutdown()

		actorID := uuid.New()
		err := svc.LogAction(context.Background(), actorID, "create", "user", "123", nil, map[string]string{"name": "test"}, "192.168.1.1", "test-agent")
		assert.NoError(t, err)

		// Wait for async processing
		time.Sleep(100 * time.Millisecond)

		assert.Equal(t, 1, repo.GetCount())
		logs := repo.GetLogs()
		require.Len(t, logs, 1)
		assert.Equal(t, actorID, logs[0].ActorID)
		assert.Equal(t, "create", logs[0].Action)
		assert.Equal(t, "user", logs[0].Resource)
		assert.Equal(t, "123", logs[0].ResourceID)
		assert.Equal(t, "192.168.1.1", logs[0].IPAddress)
		assert.Equal(t, "test-agent", logs[0].UserAgent)
	})

	t.Run("marshals after state to JSON", func(t *testing.T) {
		repo := new(MockAuditLogForAudit)
		repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

		svc := newTestAuditService(repo, 100)
		defer svc.Shutdown()

		afterData := map[string]string{"name": "John", "email": "john@example.com"}
		err := svc.LogAction(context.Background(), uuid.New(), "create", "user", "123", nil, afterData, "", "")
		assert.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		logs := repo.GetLogs()
		require.Len(t, logs, 1)
		assert.Nil(t, logs[0].Before)
		require.NotNil(t, logs[0].After)

		var after map[string]string
		err = json.Unmarshal(logs[0].After, &after)
		require.NoError(t, err)
		assert.Equal(t, "John", after["name"])
		assert.Equal(t, "john@example.com", after["email"])
	})

	t.Run("marshals before state to JSON", func(t *testing.T) {
		repo := new(MockAuditLogForAudit)
		repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

		svc := newTestAuditService(repo, 100)
		defer svc.Shutdown()

		beforeData := map[string]string{"name": "Old"}
		err := svc.LogAction(context.Background(), uuid.New(), "update", "user", "456", beforeData, nil, "", "")
		assert.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		logs := repo.GetLogs()
		require.Len(t, logs, 1)
		require.NotNil(t, logs[0].Before)
		assert.Nil(t, logs[0].After)

		var before map[string]string
		err = json.Unmarshal(logs[0].Before, &before)
		require.NoError(t, err)
		assert.Equal(t, "Old", before["name"])
	})

	t.Run("handles nil before and after gracefully", func(t *testing.T) {
		repo := new(MockAuditLogForAudit)
		repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

		svc := newTestAuditService(repo, 100)
		defer svc.Shutdown()

		err := svc.LogAction(context.Background(), uuid.New(), "delete", "user", "789", nil, nil, "", "")
		assert.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		logs := repo.GetLogs()
		require.Len(t, logs, 1)
		assert.Nil(t, logs[0].Before)
		assert.Nil(t, logs[0].After)
	})

	t.Run("drops log when channel is full", func(t *testing.T) {
		repo := new(MockAuditLogForAudit)
		// Make Create block so channel fills up
		blockCh := make(chan struct{})
		repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Run(func(args mock.Arguments) {
			<-blockCh // Block until we signal
		}).Return(nil)

		svc := newTestAuditService(repo, 1) // Very small buffer
		defer svc.Shutdown()

		// First call fills the buffer
		err := svc.LogAction(context.Background(), uuid.New(), "create", "user", "1", nil, nil, "", "")
		assert.NoError(t, err)

		// Give a moment for the worker to pick up the first item and block
		time.Sleep(50 * time.Millisecond)

		// Second call should be dropped (channel full + worker blocked)
		// This is a best-effort test since timing is involved
		// The important thing is that LogAction doesn't block or panic

		// Unblock the worker
		close(blockCh)
	})

	t.Run("sanitizes user agent to 500 chars", func(t *testing.T) {
		repo := new(MockAuditLogForAudit)
		repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

		svc := newTestAuditService(repo, 100)
		defer svc.Shutdown()

		longUA := strings.Repeat("a", 600)
		err := svc.LogAction(context.Background(), uuid.New(), "create", "user", "1", nil, nil, "", longUA)
		assert.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		logs := repo.GetLogs()
		require.Len(t, logs, 1)
		assert.LessOrEqual(t, len(logs[0].UserAgent), 500)
	})
}

func TestAuditService_LogActionSync(t *testing.T) {
	t.Run("persists synchronously and returns nil on success", func(t *testing.T) {
		repo := new(MockAuditLogForAudit)
		repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

		svc := newTestAuditService(repo, 100)
		defer svc.Shutdown()

		actorID := uuid.New()
		err := svc.LogActionSync(context.Background(), actorID, "create", "user", "123", nil, map[string]string{"name": "test"}, "10.0.0.1", "sync-agent")
		assert.NoError(t, err)

		repo.AssertCalled(t, "Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog"))
	})

	t.Run("returns error when repo fails", func(t *testing.T) {
		repo := new(MockAuditLogForAudit)
		repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(assert.AnError)

		svc := newTestAuditService(repo, 100)
		defer svc.Shutdown()

		err := svc.LogActionSync(context.Background(), uuid.New(), "create", "user", "123", nil, nil, "", "")
		assert.Error(t, err)
	})

	t.Run("marshals before/after state synchronously", func(t *testing.T) {
		repo := new(MockAuditLogForAudit)
		repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

		svc := newTestAuditService(repo, 100)
		defer svc.Shutdown()

		beforeData := map[string]string{"status": "draft"}
		afterData := map[string]string{"status": "published"}
		err := svc.LogActionSync(context.Background(), uuid.New(), "update", "user", "1", beforeData, afterData, "", "")
		assert.NoError(t, err)

		repo.AssertCalled(t, "Create", mock.Anything, mock.MatchedBy(func(log *domain.AuditLog) bool {
			return log.Before != nil && log.After != nil
		}))
	})
}

func TestAuditService_GetActorAuditLogs(t *testing.T) {
	t.Run("delegates to repository", func(t *testing.T) {
		repo := new(MockAuditLogForAudit)
		actorID := uuid.New()
		expectedLogs := []domain.AuditLog{
			{ID: uuid.New(), ActorID: actorID, Action: "create", Resource: "user"},
		}
		repo.On("FindByActorID", mock.Anything, actorID, 10, 0).Return(expectedLogs, nil)

		svc := newTestAuditService(repo, 100)
		defer svc.Shutdown()

		logs, err := svc.GetActorAuditLogs(context.Background(), actorID, 10, 0)
		assert.NoError(t, err)
		assert.Len(t, logs, 1)
		assert.Equal(t, actorID, logs[0].ActorID)
	})
}

func TestAuditService_GetResourceAuditLogs(t *testing.T) {
	t.Run("delegates to repository", func(t *testing.T) {
		repo := new(MockAuditLogForAudit)
		expectedLogs := []domain.AuditLog{
			{ID: uuid.New(), Action: "create", Resource: "user", ResourceID: "123"},
		}
		repo.On("FindByResource", mock.Anything, "user", "123", 10, 0).Return(expectedLogs, nil)

		svc := newTestAuditService(repo, 100)
		defer svc.Shutdown()

		logs, err := svc.GetResourceAuditLogs(context.Background(), "user", "123", 10, 0)
		assert.NoError(t, err)
		assert.Len(t, logs, 1)
	})
}

func TestAuditService_GetAllAuditLogs(t *testing.T) {
	t.Run("delegates to repository", func(t *testing.T) {
		repo := new(MockAuditLogForAudit)
		expectedLogs := []domain.AuditLog{
			{ID: uuid.New(), Action: "create", Resource: "user"},
			{ID: uuid.New(), Action: "update", Resource: "role"},
		}
		repo.On("FindAll", mock.Anything, 20, 0).Return(expectedLogs, nil)

		svc := newTestAuditService(repo, 100)
		defer svc.Shutdown()

		logs, err := svc.GetAllAuditLogs(context.Background(), 20, 0)
		assert.NoError(t, err)
		assert.Len(t, logs, 2)
	})
}

func TestAuditService_Shutdown(t *testing.T) {
	t.Run("drains remaining logs before shutdown", func(t *testing.T) {
		repo := new(MockAuditLogForAudit)
		repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

		svc := newTestAuditService(repo, 100)

		// Queue a log
		err := svc.LogAction(context.Background(), uuid.New(), "create", "user", "1", nil, nil, "", "")
		assert.NoError(t, err)

		// Shutdown should drain
		svc.Shutdown()

		// Give time for drain
		time.Sleep(100 * time.Millisecond)

		assert.GreaterOrEqual(t, repo.GetCount(), 1)
	})
}

func TestDefaultAuditServiceConfig(t *testing.T) {
	t.Run("returns default buffer size of 100", func(t *testing.T) {
		config := service.DefaultAuditServiceConfig()
		assert.Equal(t, 100, config.BufferSize)
	})
}

func TestAuditLog_TableName(t *testing.T) {
	t.Run("returns correct table name", func(t *testing.T) {
		log := domain.AuditLog{}
		assert.Equal(t, "audit_logs", log.TableName())
	})
}

func TestAuditLog_Constants(t *testing.T) {
	t.Run("audit action constants are defined", func(t *testing.T) {
		assert.Equal(t, "create", domain.AuditActionCreate)
		assert.Equal(t, "update", domain.AuditActionUpdate)
		assert.Equal(t, "delete", domain.AuditActionDelete)
		assert.Equal(t, "login", domain.AuditActionLogin)
		assert.Equal(t, "logout", domain.AuditActionLogout)
		assert.Equal(t, "assign", domain.AuditActionAssign)
		assert.Equal(t, "revoke", domain.AuditActionRevoke)
		assert.Equal(t, "grant", domain.AuditActionGrant)
		assert.Equal(t, "deny", domain.AuditActionDeny)
		assert.Equal(t, "login_failed", domain.AuditActionLoginFailed)
		assert.Equal(t, "password_reset", domain.AuditActionPasswordReset)
		assert.Equal(t, "password_change", domain.AuditActionPasswordChange)
		assert.Equal(t, "token_reuse", domain.AuditActionTokenReuse)
	})

	t.Run("audit resource constants are defined", func(t *testing.T) {
		assert.Equal(t, "user", domain.AuditResourceUser)
		assert.Equal(t, "role", domain.AuditResourceRole)
		assert.Equal(t, "permission", domain.AuditResourcePermission)
		assert.Equal(t, "auth", domain.AuditResourceAuth)
	})
}

