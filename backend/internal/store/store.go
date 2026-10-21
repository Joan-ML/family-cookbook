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
