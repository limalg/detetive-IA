package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"detetive-ia/internal/models"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidInput       = errors.New("username and password must not be empty (min 3 chars username, min 4 chars password)")
	ErrUserExists         = errors.New("username already taken")
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrSessionNotFound    = errors.New("invalid or expired session token")
)

// Service provides user registration, login, and session validation.
type Service struct {
	db *sql.DB
}

// New creates an authentication service instance backed by SQLite.
func New(db *sql.DB) *Service {
	return &Service{db: db}
}

// Register creates a new user account with a bcrypt-hashed password.
func (s *Service) Register(username, password string) (*models.User, error) {
	username = strings.TrimSpace(username)
	if len(username) < 3 || len(password) < 4 {
		return nil, ErrInvalidInput
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	now := time.Now().UTC()
	query := `INSERT INTO users (username, password_hash, created_at) VALUES (?, ?, ?)`
	res, err := s.db.Exec(query, username, string(hash), now.Format(time.RFC3339))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, ErrUserExists
		}
		return nil, fmt.Errorf("failed to insert user: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get inserted user id: %w", err)
	}

	return &models.User{
		ID:           id,
		Username:     username,
		PasswordHash: string(hash),
		CreatedAt:    now,
	}, nil
}

// Login verifies user credentials and returns a secure session token.
func (s *Service) Login(username, password string) (string, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return "", ErrInvalidCredentials
	}

	var userID int64
	var passwordHash string
	query := `SELECT id, password_hash FROM users WHERE username = ?`
	err := s.db.QueryRow(query, username).Scan(&userID, &passwordHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrInvalidCredentials
		}
		return "", fmt.Errorf("failed to query user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return "", ErrInvalidCredentials
	}

	// Generate 32-byte cryptographically secure random session token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	expiresAt := time.Now().UTC().Add(24 * time.Hour)
	insertSession := `INSERT INTO sessions (user_id, token, expires_at) VALUES (?, ?, ?)`
	if _, err := s.db.Exec(insertSession, userID, token, expiresAt.Format(time.RFC3339)); err != nil {
		return "", fmt.Errorf("failed to save session: %w", err)
	}

	return token, nil
}

// ValidateSession returns the authenticated user for a valid, unexpired token.
func (s *Service) ValidateSession(token string) (*models.User, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrSessionNotFound
	}

	var user models.User
	var createdAtStr, expiresAtStr string

	query := `
		SELECT u.id, u.username, u.created_at, s.expires_at
		FROM sessions s
		JOIN users u ON s.user_id = u.id
		WHERE s.token = ?
	`
	err := s.db.QueryRow(query, token).Scan(&user.ID, &user.Username, &createdAtStr, &expiresAtStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to query session: %w", err)
	}

	expiresAt, err := parseDBTime(expiresAtStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse expiration date: %w", err)
	}

	if time.Now().UTC().After(expiresAt) {
		_ = s.Logout(token)
		return nil, ErrSessionNotFound
	}

	user.CreatedAt, _ = parseDBTime(createdAtStr)
	return &user, nil
}

// Logout removes the session token, invalidating subsequent requests.
func (s *Service) Logout(token string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

// Package-level helpers
func Register(database *sql.DB, username, password string) (*models.User, error) {
	return New(database).Register(username, password)
}

func Login(database *sql.DB, username, password string) (string, error) {
	return New(database).Login(username, password)
}

func ValidateSession(database *sql.DB, token string) (*models.User, error) {
	return New(database).ValidateSession(token)
}

func Logout(database *sql.DB, token string) error {
	return New(database).Logout(token)
}

func parseDBTime(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02 15:04:05", s)
}
