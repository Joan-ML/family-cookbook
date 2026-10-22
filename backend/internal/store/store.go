// Package store defines the storage interface and domain types, independent
// of both the HTTP API and the database.
package store

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned when a requested item doesn't exist.
var ErrNotFound = errors.New("not found")

// RecipeContent holds the fields of a recipe that users edit.
type RecipeContent struct {
	Name        string
	Ingredients []string
	Steps       []string
	Tags        []string
	Source      string // empty if unknown
}

// Recipe is a stored recipe.
type Recipe struct {
	ID string
	RecipeContent
	CreatedAt time.Time
	CreatedBy string
	UpdatedAt time.Time
	UpdatedBy string
}

// RecipeSummary holds the fields needed to show a recipe in a list.
type RecipeSummary struct {
	ID   string
	Name string
	Tags []string
}

// Store persists recipes.
type Store interface {
	// ListRecipes returns all recipes, sorted by name.
	ListRecipes(ctx context.Context) ([]RecipeSummary, error)
	GetRecipe(ctx context.Context, id string) (Recipe, error)
	CreateRecipe(ctx context.Context, r Recipe) error
	// UpdateRecipe replaces a recipe's content and records who changed it.
	UpdateRecipe(ctx context.Context, id string, c RecipeContent, by string, at time.Time) (Recipe, error)
	DeleteRecipe(ctx context.Context, id string) error
}

// ErrExists is returned when creating something that already exists.
var ErrExists = errors.New("already exists")

// User is an account. Usernames are stored normalized (see package auth).
type User struct {
	ID           int64
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

// Users persists accounts.
type Users interface {
	// CreateUser returns ErrExists if the username is taken.
	CreateUser(ctx context.Context, username, passwordHash string, at time.Time) error
	GetUser(ctx context.Context, username string) (User, error)
	// ListUsers returns all users, sorted by username.
	ListUsers(ctx context.Context) ([]User, error)
	// SetPassword changes a user's password and ends all their sessions.
	SetPassword(ctx context.Context, username, passwordHash string) error
	// DeleteUser removes a user and ends all their sessions.
	DeleteUser(ctx context.Context, username string) error
}

// Session is a logged-in browser. Only a hash of its token is stored.
type Session struct {
	TokenHash string
	UserID    int64
	Username  string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Sessions persists login sessions.
type Sessions interface {
	CreateSession(ctx context.Context, s Session) error
	// GetSession returns the session with its user's current username.
	GetSession(ctx context.Context, tokenHash string) (Session, error)
	ExtendSession(ctx context.Context, tokenHash string, expiresAt time.Time) error
	DeleteSession(ctx context.Context, tokenHash string) error
	DeleteExpiredSessions(ctx context.Context, now time.Time) error
}
