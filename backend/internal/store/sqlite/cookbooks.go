package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"family-cookbook/backend/internal/store"
)

func (s *Store) ListCookbooks(ctx context.Context, userID int64) ([]store.CookbookSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.name, m.role,
		       (SELECT count(*) FROM recipes r WHERE r.cookbook_id = c.id)
		FROM cookbooks c JOIN memberships m ON m.cookbook_id = c.id
		WHERE m.user_id = ?
		ORDER BY c.name COLLATE NOCASE, c.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cookbooks := []store.CookbookSummary{}
	for rows.Next() {
		var c store.CookbookSummary
		if err := rows.Scan(&c.ID, &c.Name, &c.Role, &c.RecipeCount); err != nil {
			return nil, err
		}
		cookbooks = append(cookbooks, c)
	}
	return cookbooks, rows.Err()
}

func (s *Store) CreateCookbook(ctx context.Context, c store.Cookbook, adminID int64) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO cookbooks (id, name, created_at, created_by) VALUES (?, ?, ?, ?)`,
			c.ID, c.Name, formatTime(c.CreatedAt), c.CreatedBy)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx,
			`INSERT INTO memberships (cookbook_id, user_id, role, joined_at) VALUES (?, ?, ?, ?)`,
			c.ID, adminID, store.RoleAdmin, formatTime(c.CreatedAt))
		return err
	})
}

func (s *Store) GetCookbook(ctx context.Context, id string, userID int64) (store.Cookbook, error) {
	var c store.Cookbook
	var createdAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT c.id, c.name, c.created_at, c.created_by, m.role
		FROM cookbooks c JOIN memberships m ON m.cookbook_id = c.id
		WHERE c.id = ? AND m.user_id = ?`, id, userID).
		Scan(&c.ID, &c.Name, &createdAt, &c.CreatedBy, &c.Role)
	if errors.Is(err, sql.ErrNoRows) {
		return store.Cookbook{}, store.ErrNotFound
	}
	if err != nil {
		return store.Cookbook{}, err
	}
	c.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	return c, err
}

func (s *Store) RenameCookbook(ctx context.Context, id, name string) error {
	return expectOneRow(s.db.ExecContext(ctx, `UPDATE cookbooks SET name = ? WHERE id = ?`, name, id))
}

func (s *Store) DeleteCookbook(ctx context.Context, id string) error {
	// Recipes, their tags and memberships go with it (ON DELETE CASCADE).
	return expectOneRow(s.db.ExecContext(ctx, `DELETE FROM cookbooks WHERE id = ?`, id))
}

func (s *Store) AddMember(ctx context.Context, cookbookID string, userID int64, role store.Role, at time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO memberships (cookbook_id, user_id, role, joined_at) VALUES (?, ?, ?, ?)`,
		cookbookID, userID, role, formatTime(at))
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return store.ErrExists
	}
	return err
}

func (s *Store) DeleteUser(ctx context.Context, username string) (store.UserRemoval, error) {
	removal := store.UserRemoval{Promoted: map[string]string{}}
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		var userID int64
		err := tx.QueryRowContext(ctx, `SELECT id FROM users WHERE username = ?`, username).Scan(&userID)
		if errors.Is(err, sql.ErrNoRows) {
			return store.ErrNotFound
		}
		if err != nil {
			return err
		}

		// For each of the user's cookbooks: how many other members, and
		// whether another admin remains once the user is gone.
		type membership struct {
			cookbookID, name string
			others           int
			otherAdmins      int
		}
		rows, err := tx.QueryContext(ctx, `
			SELECT c.id, c.name,
			       (SELECT count(*) FROM memberships o WHERE o.cookbook_id = c.id AND o.user_id != ?),
			       (SELECT count(*) FROM memberships o WHERE o.cookbook_id = c.id AND o.user_id != ? AND o.role = 'admin')
			FROM cookbooks c JOIN memberships m ON m.cookbook_id = c.id
			WHERE m.user_id = ?
			ORDER BY c.name COLLATE NOCASE`, userID, userID, userID)
		if err != nil {
			return err
		}
		var memberships []membership
		for rows.Next() {
			var m membership
			if err := rows.Scan(&m.cookbookID, &m.name, &m.others, &m.otherAdmins); err != nil {
				rows.Close()
				return err
			}
			memberships = append(memberships, m)
		}
		if err := rows.Close(); err != nil {
			return err
		}

		for _, m := range memberships {
			switch {
			case m.others == 0:
				if _, err := tx.ExecContext(ctx, `DELETE FROM cookbooks WHERE id = ?`, m.cookbookID); err != nil {
					return err
				}
				removal.DeletedCookbooks = append(removal.DeletedCookbooks, m.name)
			case m.otherAdmins == 0:
				var promoted string
				err := tx.QueryRowContext(ctx, `
					UPDATE memberships SET role = 'admin'
					WHERE cookbook_id = ?1 AND user_id = (
						SELECT user_id FROM memberships
						WHERE cookbook_id = ?1 AND user_id != ?2
						ORDER BY joined_at, user_id LIMIT 1)
					RETURNING (SELECT username FROM users WHERE id = memberships.user_id)`,
					m.cookbookID, userID).Scan(&promoted)
				if err != nil {
					return err
				}
				removal.Promoted[m.name] = promoted
			}
		}

		// Sessions and memberships go with the user (ON DELETE CASCADE).
		_, err = tx.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, userID)
		return err
	})
	return removal, err
}

// expectOneRow turns "no row affected" into ErrNotFound.
func expectOneRow(res sql.Result, err error) error {
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

func (s *Store) ListMembers(ctx context.Context, cookbookID string) ([]store.Member, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.username, m.role, m.joined_at
		FROM memberships m JOIN users u ON u.id = m.user_id
		WHERE m.cookbook_id = ?
		ORDER BY u.username`, cookbookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	members := []store.Member{}
	for rows.Next() {
		var m store.Member
		var joinedAt string
		if err := rows.Scan(&m.Username, &m.Role, &joinedAt); err != nil {
			return nil, err
		}
		if m.JoinedAt, err = time.Parse(time.RFC3339Nano, joinedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (s *Store) SetRole(ctx context.Context, cookbookID string, userID int64, role store.Role) (store.Member, error) {
	var m store.Member
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		current, err := memberRole(ctx, tx, cookbookID, userID)
		if err != nil {
			return err
		}
		if current == store.RoleAdmin && role != store.RoleAdmin {
			if err := ensureAnotherAdmin(ctx, tx, cookbookID, userID); err != nil {
				return err
			}
		}
		var joinedAt string
		err = tx.QueryRowContext(ctx, `
			UPDATE memberships SET role = ? WHERE cookbook_id = ? AND user_id = ?
			RETURNING (SELECT username FROM users WHERE id = user_id), role, joined_at`,
			role, cookbookID, userID).Scan(&m.Username, &m.Role, &joinedAt)
		if err != nil {
			return err
		}
		m.JoinedAt, err = time.Parse(time.RFC3339Nano, joinedAt)
		return err
	})
	return m, err
}

func (s *Store) RemoveMember(ctx context.Context, cookbookID string, userID int64) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		current, err := memberRole(ctx, tx, cookbookID, userID)
		if err != nil {
			return err
		}
		if current == store.RoleAdmin {
			if err := ensureAnotherAdmin(ctx, tx, cookbookID, userID); err != nil {
				return err
			}
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM memberships WHERE cookbook_id = ? AND user_id = ?`, cookbookID, userID)
		return err
	})
}

// memberRole returns a member's role, or ErrNotFound if they aren't one.
func memberRole(ctx context.Context, tx *sql.Tx, cookbookID string, userID int64) (store.Role, error) {
	var role store.Role
	err := tx.QueryRowContext(ctx,
		`SELECT role FROM memberships WHERE cookbook_id = ? AND user_id = ?`, cookbookID, userID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return "", store.ErrNotFound
	}
	return role, err
}

// ensureAnotherAdmin returns ErrLastAdmin unless the cookbook has an admin
// besides userID. It runs in the same transaction as the change it guards,
// so two admins demoting each other at once can't both succeed.
func ensureAnotherAdmin(ctx context.Context, tx *sql.Tx, cookbookID string, userID int64) error {
	var others int
	err := tx.QueryRowContext(ctx, `
		SELECT count(*) FROM memberships
		WHERE cookbook_id = ? AND user_id != ? AND role = 'admin'`, cookbookID, userID).Scan(&others)
	if err != nil {
		return err
	}
	if others == 0 {
		return store.ErrLastAdmin
	}
	return nil
}
