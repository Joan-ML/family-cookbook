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

// ErrExists is returned when creating something that already exists.
var ErrExists = errors.New("already exists")

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
	ID         string
	CookbookID string
	RecipeContent
	CreatedAt time.Time
	CreatedBy string // username, kept as history even if the user is removed
	UpdatedAt time.Time
	UpdatedBy string
}

// RecipeSummary holds the fields needed to show a recipe in a list.
type RecipeSummary struct {
	ID   string
	Name string
	Tags []string
}

// Store persists recipes. Every method is scoped to a cookbook: a recipe
// asked for under another cookbook is ErrNotFound.
type Store interface {
	// ListRecipes returns a cookbook's recipes, sorted by name.
	ListRecipes(ctx context.Context, cookbookID string) ([]RecipeSummary, error)
	GetRecipe(ctx context.Context, cookbookID, id string) (Recipe, error)
	CreateRecipe(ctx context.Context, r Recipe) error
	// UpdateRecipe replaces a recipe's content and records who changed it.
	UpdateRecipe(ctx context.Context, cookbookID, id string, c RecipeContent, by string, at time.Time) (Recipe, error)
	DeleteRecipe(ctx context.Context, cookbookID, id string) error
}

// Role is a member's role in a cookbook.
type Role string

const (
	// RoleAdmin can also manage members, rename and delete the cookbook.
	RoleAdmin Role = "admin"
	// RoleEditor can create, edit and delete the cookbook's recipes.
	RoleEditor Role = "editor"
)

// Cookbook is a cookbook as seen by one of its members.
type Cookbook struct {
	ID        string
	Name      string
	CreatedAt time.Time
	CreatedBy string
	Role      Role // the member's role
}

// CookbookSummary holds the fields needed to show a cookbook in a list.
type CookbookSummary struct {
	ID          string
	Name        string
	Role        Role
	RecipeCount int
}

// Cookbooks persists cookbooks and who belongs to them. Lookups take the
// member's user ID: for anyone else, a cookbook is ErrNotFound.
type Cookbooks interface {
	// ListCookbooks returns the user's cookbooks, sorted by name.
	ListCookbooks(ctx context.Context, userID int64) ([]CookbookSummary, error)
	// CreateCookbook creates a cookbook with the user as its admin.
	CreateCookbook(ctx context.Context, c Cookbook, adminID int64) error
	GetCookbook(ctx context.Context, id string, userID int64) (Cookbook, error)
	RenameCookbook(ctx context.Context, id, name string) error
	// DeleteCookbook deletes a cookbook with all its recipes.
	DeleteCookbook(ctx context.Context, id string) error
	// AddMember adds a user to a cookbook, or returns ErrExists.
	AddMember(ctx context.Context, cookbookID string, userID int64, role Role, at time.Time) error
}

// User is an account. Usernames are stored normalized (see package auth).
type User struct {
	ID           int64
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

// UserRemoval reports what removing a user changed in their cookbooks.
type UserRemoval struct {
	// DeletedCookbooks are the cookbooks the user was the only member of.
	DeletedCookbooks []string
	// Promoted maps cookbook names to the member who became admin because
	// the removed user was the cookbook's last admin.
	Promoted map[string]string
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
	// DeleteUser removes a user, their sessions and their memberships. Their
	// cookbooks never become admin-less: a cookbook they were the only member
	// of is deleted, and if they were the last admin, the longest-standing
	// remaining member becomes admin.
	DeleteUser(ctx context.Context, username string) (UserRemoval, error)
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
