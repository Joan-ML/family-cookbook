// Package sqlite implements store.Store on top of SQLite.
package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"sort"
	"time"

	"family-cookbook/backend/internal/store"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Store is a SQLite-backed store.Store.
type Store struct {
	db *sql.DB
}

var _ store.Store = (*Store)(nil)

// Open opens (creating if needed) the database file at path and applies any
// pending migrations.
func Open(path string) (*Store, error) {
	// WAL and a busy timeout let the server and admin commands share the file.
	dsn := "file:" + (&url.URL{Path: path}).EscapedPath() +
		"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrating database: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// migrate applies migrations/NNN_*.sql in order, tracking progress in
// PRAGMA user_version.
func (s *Store) migrate(ctx context.Context) error {
	names, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)

	var version int
	if err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	for i := version; i < len(names); i++ {
		script, err := migrations.ReadFile(names[i])
		if err != nil {
			return err
		}
		err = s.withTx(ctx, func(tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, string(script)); err != nil {
				return fmt.Errorf("%s: %w", names[i], err)
			}
			_, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", i+1))
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListRecipes(ctx context.Context) ([]store.RecipeSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id, r.name,
		       (SELECT json_group_array(t.tag ORDER BY t.position)
		        FROM recipe_tags t WHERE t.recipe_id = r.id)
		FROM recipes r
		ORDER BY r.name COLLATE NOCASE, r.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summaries := []store.RecipeSummary{}
	for rows.Next() {
		var sum store.RecipeSummary
		var tags string
		if err := rows.Scan(&sum.ID, &sum.Name, &tags); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(tags), &sum.Tags); err != nil {
			return nil, err
		}
		summaries = append(summaries, sum)
	}
	return summaries, rows.Err()
}

func (s *Store) GetRecipe(ctx context.Context, id string) (store.Recipe, error) {
	return getRecipe(ctx, s.db, id)
}

func (s *Store) CreateRecipe(ctx context.Context, r store.Recipe) error {
	ingredients, steps, err := marshalLists(r.RecipeContent)
	if err != nil {
		return err
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO recipes (id, name, source, ingredients, steps,
			                     created_at, created_by, updated_at, updated_by)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			r.ID, r.Name, r.Source, ingredients, steps,
			formatTime(r.CreatedAt), r.CreatedBy, formatTime(r.UpdatedAt), r.UpdatedBy)
		if err != nil {
			return err
		}
		return replaceTags(ctx, tx, r.ID, r.Tags)
	})
}

func (s *Store) UpdateRecipe(ctx context.Context, id string, c store.RecipeContent, by string, at time.Time) (store.Recipe, error) {
	ingredients, steps, err := marshalLists(c)
	if err != nil {
		return store.Recipe{}, err
	}
	var updated store.Recipe
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE recipes
			SET name = ?, source = ?, ingredients = ?, steps = ?, updated_at = ?, updated_by = ?
			WHERE id = ?`,
			c.Name, c.Source, ingredients, steps, formatTime(at), by, id)
		if err != nil {
			return err
		}
		if n, err := res.RowsAffected(); err != nil {
			return err
		} else if n == 0 {
			return store.ErrNotFound
		}
		if err := replaceTags(ctx, tx, id, c.Tags); err != nil {
			return err
		}
		updated, err = getRecipe(ctx, tx, id)
		return err
	})
	return updated, err
}

func (s *Store) DeleteRecipe(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM recipes WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

// querier is satisfied by both *sql.DB and *sql.Tx.
type querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func getRecipe(ctx context.Context, q querier, id string) (store.Recipe, error) {
	var r store.Recipe
	var ingredients, steps, tags, createdAt, updatedAt string
	err := q.QueryRowContext(ctx, `
		SELECT id, name, source, ingredients, steps,
		       created_at, created_by, updated_at, updated_by,
		       (SELECT json_group_array(t.tag ORDER BY t.position)
		        FROM recipe_tags t WHERE t.recipe_id = recipes.id)
		FROM recipes WHERE id = ?`, id).
		Scan(&r.ID, &r.Name, &r.Source, &ingredients, &steps,
			&createdAt, &r.CreatedBy, &updatedAt, &r.UpdatedBy, &tags)
	if errors.Is(err, sql.ErrNoRows) {
		return store.Recipe{}, store.ErrNotFound
	}
	if err != nil {
		return store.Recipe{}, err
	}

	for _, f := range []struct {
		src string
		dst *[]string
	}{{ingredients, &r.Ingredients}, {steps, &r.Steps}, {tags, &r.Tags}} {
		if err := json.Unmarshal([]byte(f.src), f.dst); err != nil {
			return store.Recipe{}, err
		}
	}
	if r.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt); err != nil {
		return store.Recipe{}, err
	}
	if r.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt); err != nil {
		return store.Recipe{}, err
	}
	return r, nil
}

func replaceTags(ctx context.Context, tx *sql.Tx, recipeID string, tags []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM recipe_tags WHERE recipe_id = ?`, recipeID); err != nil {
		return err
	}
	for i, tag := range tags {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO recipe_tags (recipe_id, tag, position) VALUES (?, ?, ?)`, recipeID, tag, i)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) withTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func marshalLists(c store.RecipeContent) (ingredients, steps string, err error) {
	i, err := json.Marshal(nonNil(c.Ingredients))
	if err != nil {
		return "", "", err
	}
	s, err := json.Marshal(nonNil(c.Steps))
	if err != nil {
		return "", "", err
	}
	return string(i), string(s), nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}
