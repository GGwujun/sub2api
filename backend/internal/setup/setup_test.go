package setup

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestDecideAdminBootstrap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		totalUsers int64
		adminUsers int64
		should     bool
		reason     string
	}{
		{
			name:       "empty database should create admin",
			totalUsers: 0,
			adminUsers: 0,
			should:     true,
			reason:     adminBootstrapReasonEmptyDatabase,
		},
		{
			name:       "admin exists should skip",
			totalUsers: 10,
			adminUsers: 1,
			should:     false,
			reason:     adminBootstrapReasonAdminExists,
		},
		{
			name:       "users exist without admin should skip",
			totalUsers: 5,
			adminUsers: 0,
			should:     false,
			reason:     adminBootstrapReasonUsersExistWithoutAdmin,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := decideAdminBootstrap(tc.totalUsers, tc.adminUsers)
			if got.shouldCreate != tc.should {
				t.Fatalf("shouldCreate=%v, want %v", got.shouldCreate, tc.should)
			}
			if got.reason != tc.reason {
				t.Fatalf("reason=%q, want %q", got.reason, tc.reason)
			}
		})
	}
}

func TestSetupDefaultAdminConcurrency(t *testing.T) {
	t.Run("simple mode admin uses higher concurrency", func(t *testing.T) {
		t.Setenv("RUN_MODE", "simple")
		if got := setupDefaultAdminConcurrency(); got != simpleModeAdminConcurrency {
			t.Fatalf("setupDefaultAdminConcurrency()=%d, want %d", got, simpleModeAdminConcurrency)
		}
	})

	t.Run("standard mode keeps existing default", func(t *testing.T) {
		t.Setenv("RUN_MODE", "standard")
		if got := setupDefaultAdminConcurrency(); got != defaultUserConcurrency {
			t.Fatalf("setupDefaultAdminConcurrency()=%d, want %d", got, defaultUserConcurrency)
		}
	})
}

func TestWriteConfigFileKeepsDefaultUserConcurrency(t *testing.T) {
	t.Setenv("RUN_MODE", "simple")
	t.Setenv("DATA_DIR", t.TempDir())

	if err := writeConfigFile(&SetupConfig{}); err != nil {
		t.Fatalf("writeConfigFile() error = %v", err)
	}

	data, err := os.ReadFile(GetConfigFilePath())
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if !strings.Contains(string(data), "user_concurrency: 5") {
		t.Fatalf("config missing default user concurrency, got:\n%s", string(data))
	}
}

func TestValidateSetupSchema(t *testing.T) {
	t.Run("passes when token schema columns exist", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("sqlmock.New() error = %v", err)
		}
		defer func() { _ = db.Close() }()

		for _, item := range setupRequiredSchemaColumns {
			mock.ExpectQuery("SELECT EXISTS ").
				WithArgs(item.table, item.column).
				WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
		}

		if err := validateSetupSchema(context.Background(), db); err != nil {
			t.Fatalf("validateSetupSchema() error = %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("ExpectationsWereMet() error = %v", err)
		}
	})

	t.Run("fails when token schema column is missing", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("sqlmock.New() error = %v", err)
		}
		defer func() { _ = db.Close() }()

		first := setupRequiredSchemaColumns[0]
		mock.ExpectQuery("SELECT EXISTS ").
			WithArgs(first.table, first.column).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

		err = validateSetupSchema(context.Background(), db)
		if err == nil {
			t.Fatalf("validateSetupSchema() error = nil, want missing schema error")
		}
		if !strings.Contains(err.Error(), first.table+"."+first.column) {
			t.Fatalf("validateSetupSchema() error = %v, want missing column reference", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("ExpectationsWereMet() error = %v", err)
		}
	})
}
