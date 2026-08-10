package models

import (
	"context"
	"crypto/pbkdf2"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID          int
	Name        string
	IsSuperuser bool
}

type UserModel struct {
	DB *pgxpool.Pool
}

const authenticateQuery = `
SELECT id, username, password, is_superuser
FROM auth_user
WHERE username = $1 AND is_active`

func (m *UserModel) Authenticate(ctx context.Context, name, password string) (User, error) {
	var u User
	var hash string

	err := m.DB.QueryRow(ctx, authenticateQuery, name).Scan(&u.ID, &u.Name, &hash, &u.IsSuperuser)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, err
	}

	ok, err := verifyPassword(password, hash)
	if err != nil {
		return User{}, err
	}
	if !ok {
		return User{}, ErrInvalidCredentials
	}

	return u, nil
}

const getUserQuery = `
SELECT id, username, is_superuser
FROM auth_user
WHERE id = $1 AND is_active`

func (m *UserModel) Get(ctx context.Context, id int) (User, error) {
	var u User

	err := m.DB.QueryRow(ctx, getUserQuery, id).Scan(&u.ID, &u.Name, &u.IsSuperuser)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNoRecord
	}

	return u, err
}

func verifyPassword(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2_sha256" {
		return false, fmt.Errorf("models: unsupported password hash format")
	}

	iterations, err := strconv.Atoi(parts[1])
	if err != nil {
		return false, err
	}

	want, err := base64.StdEncoding.DecodeString(parts[3])
	if err != nil {
		return false, err
	}

	got, err := pbkdf2.Key(sha256.New, password, []byte(parts[2]), iterations, sha256.Size)
	if err != nil {
		return false, err
	}

	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
