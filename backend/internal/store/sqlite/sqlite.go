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
	"os"
	"path/filepath"
	"sort"
	"strings"
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

var (
	_ store.Store     = (*Store)(nil)
	_ store.Cookbooks = (*Store)(nil)
)

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

// Backup writes a consistent copy of the database to path while it stays in
// use, and checks the copy before keeping it. It refuses to overwrite an
// existing file.
func (s *Store) Backup(ctx context.Context, path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", path)
	}
	// Write next to the destination, then rename: a failed backup never
	// leaves a partial file behind under the final name.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".backup-*.db")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	os.Remove(tmpPath) // VACUUM INTO needs the target not to exist
	defer os.Remove(tmpPath)

	if _, err := s.db.ExecContext(ctx, `VACUUM INTO ?`, tmpPath); err != nil {
		return fmt.Errorf("writing backup: %w", err)
	}
	if err := checkIntegrity(ctx, tmpPath); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// checkIntegrity runs SQLite's integrity check on the database file at path.
func checkIntegrity(ctx context.Context, path string) error {
	db, err := sql.Open("sqlite", "file:"+(&url.URL{Path: path}).EscapedPath()+"?mode=ro")
	if err != nil {
		return err
	}
	defer db.Close()
	var result string
	if err := db.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&result); err != nil {
		return fmt.Errorf("checking backup: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("backup failed its integrity check: %s", result)
	}
	return nil
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

func (s *Store) ListRecipes(ctx context.Context, cookbookID string) ([]store.RecipeSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id, r.name,
		       (SELECT json_group_array(t.tag ORDER BY t.position)
		        FROM recipe_tags t WHERE t.recipe_id = r.id)
		FROM recipes r
		WHERE r.cookbook_id = ?
		ORDER BY r.name COLLATE NOCASE, r.id`, cookbookID)
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

func (s *Store) GetRecipe(ctx context.Context, cookbookID, id string) (store.Recipe, error) {
	return getRecipe(ctx, s.db, cookbookID, id)
}

func (s *Store) CreateRecipe(ctx context.Context, r store.Recipe) error {
	ingredients, steps, err := marshalLists(r.RecipeContent)
	if err != nil {
		return err
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO recipes (id, cookbook_id, name, source, ingredients, steps,
			                     created_at, created_by, updated_at, updated_by)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			r.ID, r.CookbookID, r.Name, r.Source, ingredients, steps,
			formatTime(r.CreatedAt), r.CreatedBy, formatTime(r.UpdatedAt), r.UpdatedBy)
		if err != nil {
			return err
		}
		return replaceTags(ctx, tx, r.ID, r.Tags)
	})
}

func (s *Store) UpdateRecipe(ctx context.Context, cookbookID, id string, c store.RecipeContent, by string, at time.Time) (store.Recipe, error) {
	ingredients, steps, err := marshalLists(c)
	if err != nil {
		return store.Recipe{}, err
	}
	var updated store.Recipe
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE recipes
			SET name = ?, source = ?, ingredients = ?, steps = ?, updated_at = ?, updated_by = ?
			WHERE id = ? AND cookbook_id = ?`,
			c.Name, c.Source, ingredients, steps, formatTime(at), by, id, cookbookID)
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
		updated, err = getRecipe(ctx, tx, cookbookID, id)
		return err
	})
	return updated, err
}

func (s *Store) DeleteRecipe(ctx context.Context, cookbookID, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM recipes WHERE id = ? AND cookbook_id = ?`, id, cookbookID)
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

func getRecipe(ctx context.Context, q querier, cookbookID, id string) (store.Recipe, error) {
	var r store.Recipe
	var ingredients, steps, tags, createdAt, updatedAt string
	err := q.QueryRowContext(ctx, `
		SELECT id, cookbook_id, name, source, ingredients, steps,
		       created_at, created_by, updated_at, updated_by,
		       (SELECT json_group_array(t.tag ORDER BY t.position)
		        FROM recipe_tags t WHERE t.recipe_id = recipes.id)
		FROM recipes WHERE id = ? AND cookbook_id = ?`, id, cookbookID).
		Scan(&r.ID, &r.CookbookID, &r.Name, &r.Source, &ingredients, &steps,
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

// timeFormat is RFC 3339 with all nine fractional digits, so stored times
// have a fixed width and sort correctly as text (RFC3339Nano trims zeros).
const timeFormat = "2006-01-02T15:04:05.000000000Z07:00"

func formatTime(t time.Time) string {
	return t.UTC().Format(timeFormat)
}

var (
	_ store.Users    = (*Store)(nil)
	_ store.Sessions = (*Store)(nil)
)

func (s *Store) CreateUser(ctx context.Context, username, passwordHash string, at time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO users (username, password_hash, created_at) VALUES (?, ?, ?)`,
		username, passwordHash, formatTime(at))
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return store.ErrExists
	}
	return err
}

func (s *Store) GetUser(ctx context.Context, username string) (store.User, error) {
	var u store.User
	var createdAt string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, password_hash, created_at FROM users WHERE username = ?`, username).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return store.User{}, store.ErrNotFound
	}
	if err != nil {
		return store.User{}, err
	}
	u.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	return u, err
}

func (s *Store) ListUsers(ctx context.Context) ([]store.User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, username, created_at FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []store.User{}
	for rows.Next() {
		var u store.User
		var createdAt string
		if err := rows.Scan(&u.ID, &u.Username, &createdAt); err != nil {
			return nil, err
		}
		if u.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) SetPassword(ctx context.Context, username, passwordHash string) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		var id int64
		err := tx.QueryRowContext(ctx,
			`UPDATE users SET password_hash = ? WHERE username = ? RETURNING id`, passwordHash, username).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			return store.ErrNotFound
		}
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, id)
		return err
	})
}

func (s *Store) CreateSession(ctx context.Context, sess store.Session) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		sess.TokenHash, sess.UserID, formatTime(sess.CreatedAt), formatTime(sess.ExpiresAt))
	return err
}

func (s *Store) GetSession(ctx context.Context, tokenHash string) (store.Session, error) {
	sess := store.Session{TokenHash: tokenHash}
	var createdAt, expiresAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT s.user_id, u.username, s.created_at, s.expires_at
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = ?`, tokenHash).
		Scan(&sess.UserID, &sess.Username, &createdAt, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return store.Session{}, store.ErrNotFound
	}
	if err != nil {
		return store.Session{}, err
	}
	if sess.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt); err != nil {
		return store.Session{}, err
	}
	sess.ExpiresAt, err = time.Parse(time.RFC3339Nano, expiresAt)
	return sess, err
}

func (s *Store) ExtendSession(ctx context.Context, tokenHash string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET expires_at = ? WHERE token_hash = ?`, formatTime(expiresAt), tokenHash)
	return err
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

func (s *Store) DeleteExpiredSessions(ctx context.Context, now time.Time) error {
	// Stored times are fixed-width UTC (see timeFormat), so text order is time order.
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, formatTime(now))
	return err
}
