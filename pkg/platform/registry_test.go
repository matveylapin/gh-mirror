package platform

import (
	"context"
	"errors"
	"testing"

	"github.com/nalgeon/be"
	"gh-mirror/pkg/models"
)

type testPlatform struct {
	id models.PlatformID
}

func (t *testPlatform) ID() models.PlatformID                                                    { return t.id }
func (t *testPlatform) Name() string                                                              { return "test" }
func (t *testPlatform) Configure(token string, apiURL string, webURL string, owner string) error                { return nil }
func (t *testPlatform) GetAuthenticatedUser(ctx context.Context) (string, error)                  { return "", nil }
func (t *testPlatform) ListRepositories(ctx context.Context) ([]models.Repository, error)         { return nil, nil }
func (t *testPlatform) GetRepository(ctx context.Context, owner string, repo string) (*models.Repository, error) { return nil, nil }
func (t *testPlatform) CreateRepository(ctx context.Context, name string, private bool, description string) (*models.Repository, error) { return nil, nil }
func (t *testPlatform) UpdateRepository(ctx context.Context, owner string, repo string, private bool, description string) error { return nil }
func (t *testPlatform) RepositoryExists(ctx context.Context, owner string, repo string) (bool, error) { return false, nil }
func (t *testPlatform) CloneURL(repo models.Repository) string                     { return "" }
func (t *testPlatform) CleanPullRefs(repoPath string) error                                        { return nil }

func TestCreateNonexistent(t *testing.T) {
	p, err := Create("nonexistent")
	be.True(t, p == nil)
	be.Err(t, err, ErrPlatformNotFound)
}

func TestRegisteredIDsEmpty(t *testing.T) {
	ids := RegisteredIDs()
	be.Equal(t, len(ids), 0)
}

func TestPlatformErrorError(t *testing.T) {
	for _, tc := range PlatformErrorTestCases() {
		t.Run(tc.Name, func(t *testing.T) {
			got := tc.Error.Error()
			be.Equal(t, got, tc.WantMsg)
		})
	}
}

func TestPlatformErrorCode(t *testing.T) {
	var err error = ErrPlatformNotFound
	platformErr, ok := err.(*PlatformError)
	be.True(t, ok)
	be.Equal(t, platformErr.Code, "PLATFORM_NOT_FOUND")
}

func TestErrorAs(t *testing.T) {
	for _, tc := range ErrorAsTestCases() {
		t.Run(tc.Name, func(t *testing.T) {
			var platformErr *PlatformError
			ok := errors.As(tc.Err, &platformErr)
			be.True(t, ok)
			be.Equal(t, platformErr.Code, tc.WantType.Code)
			be.Equal(t, platformErr.Message, tc.WantMsg)
		})
	}
}

func TestPlatformErrorMessage(t *testing.T) {
	err := &PlatformError{Code: "TEST_CODE", Message: "test message"}
	be.Equal(t, err.Error(), "TEST_CODE: test message")
}

func TestPlatformErrorMessageWithEmptyCode(t *testing.T) {
	err := &PlatformError{Code: "", Message: "empty code"}
	be.Equal(t, err.Error(), ": empty code")
}

func TestPlatformErrorMessageWithEmptyMessage(t *testing.T) {
	err := &PlatformError{Code: "CODE_ONLY", Message: ""}
	be.Equal(t, err.Error(), "CODE_ONLY: ")
}

func TestRegisterAndCreate(t *testing.T) {
	testID := models.PlatformID("test-platform-for-register")
	cleanup := func() { delete(globalRegistry, testID) }

	Register(testID, func() Platform {
		return &testPlatform{id: testID}
	})
	defer cleanup()

	p, err := Create(testID)
	be.True(t, err == nil)
	be.True(t, p != nil)
	be.Equal(t, p.ID(), testID)
}

func TestRegisteredIDsAfterRegister(t *testing.T) {
	testID := models.PlatformID("test-platform-for-ids")
	cleanup := func() { delete(globalRegistry, testID) }

	Register(testID, func() Platform {
		return &testPlatform{id: testID}
	})
	defer cleanup()

	ids := RegisteredIDs()
	found := false
	for _, id := range ids {
		if id == testID {
			found = true
			break
		}
	}
	be.True(t, found)
}

func BenchmarkCreateNonexistent(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = Create("nonexistent")
	}
}

func BenchmarkRegisteredIDs(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = RegisteredIDs()
	}
}

func BenchmarkPlatformErrorError(b *testing.B) {
	err := ErrPlatformNotFound
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = err.Error()
	}
}

func BenchmarkErrorAs(b *testing.B) {
	var platformErr *PlatformError
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		errors.As(ErrPlatformNotFound, &platformErr)
	}
}