package models

import (
	"context"
	"errors"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	adminHash = "pbkdf2_sha256$1500000$xT0rOS1a20sYdLxorfNVug$IbfYM7jlW/MARo2rYhVAYSMStjGUpPQFCXnapXn2O5o="
	guestHash = "pbkdf2_sha256$1500000$bFB6pZVMgh6Zo7yPv8VvAn$2GrjnMJiy6PKnlJylr53LopAFUmHvhFR3jSkx7rzYM4="
)

func TestVerifyPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		encoded  string
		want     bool
		wantErr  bool
	}{
		{"correct admin", "admin", adminHash, true, false},
		{"correct guest", "guest", guestHash, true, false},
		{"wrong password", "guest", adminHash, false, false},
		{"empty password", "", adminHash, false, false},
		{"unsupported algorithm", "admin", "bcrypt$12$salt$hash", false, true},
		{"malformed hash", "admin", "nonsense", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := verifyPassword(tt.password, tt.encoded)

			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

var userSeq atomic.Int64

func userName() string {
	return "go-test-auth-" + strconv.FormatInt(time.Now().UnixNano(), 10) + "-" + strconv.FormatInt(userSeq.Add(1), 10)
}

func userInsertAccount(t *testing.T, pool *pgxpool.Pool, hash string, isSuperuser, isActive bool) (int, string) {
	t.Helper()

	ctx := context.Background()
	username := userName()

	var id int

	const q = `
INSERT INTO auth_user (password, is_superuser, username, first_name, last_name, email, is_staff, is_active, date_joined)
VALUES ($1, $2, $3, '', '', '', false, $4, now())
RETURNING id`

	err := pool.QueryRow(ctx, q, hash, isSuperuser, username, isActive).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM auth_user WHERE id = $1`, id); err != nil {
			t.Errorf("cleanup of auth_user %d: %v", id, err)
		}
	})

	return id, username
}

func TestAuthenticate(t *testing.T) {
	pool := newTestPool(t)
	m := &UserModel{DB: pool}
	ctx := context.Background()

	t.Run("correct credentials", func(t *testing.T) {
		id, username := userInsertAccount(t, pool, adminHash, true, true)

		got, err := m.Authenticate(ctx, username, "admin")
		if err != nil {
			t.Fatalf("got error %v; want nil", err)
		}
		want := User{ID: id, Name: username, IsSuperuser: true}
		if got != want {
			t.Errorf("got %+v; want %+v", got, want)
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		_, username := userInsertAccount(t, pool, adminHash, false, true)

		_, err := m.Authenticate(ctx, username, "not-admin")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("got %v; want ErrInvalidCredentials", err)
		}
	})

	t.Run("unknown username", func(t *testing.T) {
		_, err := m.Authenticate(ctx, userName(), "admin")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("got %v; want ErrInvalidCredentials, so the login form cannot reveal which usernames exist", err)
		}
	})

	t.Run("inactive user", func(t *testing.T) {
		_, username := userInsertAccount(t, pool, adminHash, false, false)

		_, err := m.Authenticate(ctx, username, "admin")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("got %v; want ErrInvalidCredentials, a deactivated account must not log in with the right password", err)
		}
	})

	t.Run("unsupported hash format", func(t *testing.T) {
		_, username := userInsertAccount(t, pool, "bcrypt$12$salt$hash", false, true)

		_, err := m.Authenticate(ctx, username, "admin")
		if err == nil {
			t.Fatal("got nil; want an error, an unreadable stored hash is a server fault not a bad password")
		}
		if errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("got ErrInvalidCredentials; want a real error")
		}
	})
}

func TestUserGet(t *testing.T) {
	pool := newTestPool(t)
	m := &UserModel{DB: pool}
	ctx := context.Background()

	t.Run("known id", func(t *testing.T) {
		id, username := userInsertAccount(t, pool, adminHash, true, true)

		got, err := m.Get(ctx, id)
		if err != nil {
			t.Fatalf("got error %v; want nil", err)
		}
		want := User{ID: id, Name: username, IsSuperuser: true}
		if got != want {
			t.Errorf("got %+v; want %+v", got, want)
		}
	})

	t.Run("unknown id", func(t *testing.T) {
		_, err := m.Get(ctx, -1)
		if !errors.Is(err, ErrNoRecord) {
			t.Errorf("got %v; want ErrNoRecord", err)
		}
	})

	t.Run("inactive user", func(t *testing.T) {
		id, _ := userInsertAccount(t, pool, adminHash, false, false)

		_, err := m.Get(ctx, id)
		if !errors.Is(err, ErrNoRecord) {
			t.Errorf("got %v; want ErrNoRecord, which is what logs out an account deactivated mid-session", err)
		}
	})
}
