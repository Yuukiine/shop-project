package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/mattn/go-sqlite3"
	_ "github.com/mattn/go-sqlite3"

	"shop/internal/domain/models"
	"shop/internal/storage"
)

type Storage struct {
	db *sql.DB
}

func New(path string) (*Storage, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	return &Storage{db: db}, nil
}

func (s *Storage) GetProducts(ctx context.Context, limit, offset int) ([]models.Product, error) {
	stmt, err := s.db.Prepare(
		`
		SELECT p.id, p.name, p.description, p.price, p.stock, c.name
		FROM products AS p
		JOIN categories AS c ON c.id = p.category_id
		LIMIT ? OFFSET ?`)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare statement: %w", err)
	}

	var p []models.Product

	rows, err := stmt.QueryContext(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query products: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var pr models.Product

		err = rows.Scan(
			&pr.ID,
			&pr.Name,
			&pr.Description,
			&pr.Price,
			&pr.Stock,
			&pr.Category,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan product: %w", err)
		}

		p = append(p, pr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan products: %w", err)
	}

	return p, nil
}

func (s *Storage) SaveUser(ctx context.Context, email string, passHash []byte) (int64, error) {
	stmt, err := s.db.Prepare(`
		INSERT INTO users(email, pass_hash) 
		VALUES(?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare statement: %w", err)
	}

	res, err := stmt.ExecContext(ctx, email, passHash)
	if err != nil {
		var sqliteErr sqlite3.Error

		if errors.As(err, &sqliteErr) && errors.Is(sqliteErr.ExtendedCode, sqlite3.ErrConstraintUnique) {
			return 0, fmt.Errorf("user already exists: %w", storage.ErrUserExists)
		}

		return 0, fmt.Errorf("failed to insert user: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to fetch user id: %w", err)
	}

	return id, nil
}

func (s *Storage) User(ctx context.Context, email string) (models.User, error) {
	stmt, err := s.db.Prepare(`
		SELECT id, email, pass_hash 
		FROM users 
		WHERE email = ?`)
	if err != nil {
		return models.User{}, fmt.Errorf("failed to prepare statement: %w", err)
	}

	row := stmt.QueryRowContext(ctx, email)

	var user models.User
	err = row.Scan(&user.ID, &user.Email, &user.PassHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.User{}, fmt.Errorf("user not found: %w", storage.ErrUserNotFound)
		}

		return models.User{}, fmt.Errorf("failed to fetch user: %w", err)
	}

	return user, nil
}

func (s *Storage) IsAdmin(ctx context.Context, userID int64) (bool, error) {
	stmt, err := s.db.Prepare(`
		SELECT is_admin 
		FROM users 
		WHERE id = ?`)
	if err != nil {
		return false, fmt.Errorf("failed to prepare statement: %w", err)
	}

	row := stmt.QueryRowContext(ctx, userID)

	var isAdmin bool

	err = row.Scan(&isAdmin)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, fmt.Errorf("user not found: %w", storage.ErrUserNotFound)
		}

		return false, fmt.Errorf("failed to fetch user: %w", err)
	}

	return isAdmin, nil
}

func (s *Storage) App(ctx context.Context, appID int) (models.App, error) {
	stmt, err := s.db.Prepare(`
		SELECT id, name, secret 
		FROM apps 
		WHERE id = ?`)
	if err != nil {
		return models.App{}, fmt.Errorf("failed to prepare statement: %w", err)
	}

	row := stmt.QueryRowContext(ctx, appID)

	var app models.App
	err = row.Scan(&app.ID, &app.Name, &app.Secret)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.App{}, fmt.Errorf("app not found: %w", storage.ErrAppNotFound)
		}

		return models.App{}, fmt.Errorf("failed to fetch app: %w", err)
	}

	return app, nil
}
