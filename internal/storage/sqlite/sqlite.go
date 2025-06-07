package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

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

func (s *Storage) GetProduct(ctx context.Context, id int) (models.Product, error) {
	const op = "storage.GetProduct"

	stmt, err := s.db.Prepare(
		`
		SELECT p.id, p.name, p.description, p.price, p.stock, c.name
		FROM products AS p
		JOIN categories AS c ON c.id = p.category_id
		WHERE p.id = ?`)
	if err != nil {
		return models.Product{}, fmt.Errorf("failed to prepare statement: %w", err)
	}

	var p models.Product
	row := stmt.QueryRowContext(ctx, id)

	err = row.Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.Stock, &p.Category)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Product{}, fmt.Errorf("%s: product not found: %w", op, storage.ErrProductNotFound)
		}

		return models.Product{}, fmt.Errorf("%s: failed to fetch product: %w", op, err)
	}

	return p, nil
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
	const op = "storage.User"

	stmt, err := s.db.Prepare(`
		SELECT id, email, pass_hash 
		FROM users 
		WHERE email = ?`)
	if err != nil {
		return models.User{}, fmt.Errorf("%s: failed to prepare statement: %w", op, err)
	}

	row := stmt.QueryRowContext(ctx, email)

	var user models.User
	err = row.Scan(&user.ID, &user.Email, &user.PassHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.User{}, fmt.Errorf("%s: user not found: %w", op, storage.ErrUserNotFound)
		}

		return models.User{}, fmt.Errorf("%s: failed to fetch user: %w", op, err)
	}

	return user, nil
}

func (s *Storage) IsAdmin(ctx context.Context, userID int64) (bool, error) {
	const op = "storage.IsAdmin"
	stmt, err := s.db.Prepare(`
		SELECT is_admin 
		FROM users 
		WHERE user_id = ?`)
	if err != nil {
		return false, fmt.Errorf("%s: failed to prepare statement: %w", op, err)
	}

	row := stmt.QueryRowContext(ctx, userID)

	var isAdmin bool

	err = row.Scan(&isAdmin)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, fmt.Errorf("%s: user not found: %w", op, storage.ErrUserNotFound)
		}

		return false, fmt.Errorf("%s: failed to fetch user: %w", op, err)
	}

	return isAdmin, nil
}

func (s *Storage) App(ctx context.Context, appID int) (models.App, error) {
	stmt, err := s.db.Prepare(`
		SELECT app_id, name, secret 
		FROM apps 
		WHERE app_id = ?`)
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

func (s *Storage) TotalProducts(ctx context.Context) (int, error) {
	const op = "storage.TotalProducts"

	stmt, err := s.db.Prepare(`
		SELECT COUNT(id)
		FROM products`)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare statement: %w", err)
	}

	row := stmt.QueryRowContext(ctx)

	var total int

	err = row.Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("%s: failed to fetch total products: %w", op, err)
	}

	return total, nil
}

func (s *Storage) CreateSession(ctx context.Context, UUID string) error {
	const op = "storage.CreateSession"

	expiresAt := time.Now().Add(24 * time.Hour)

	stmt, err := s.db.Prepare(`
		INSERT INTO sessions (uuid, expires_at)
		VALUES(?, ?)`)
	if err != nil {
		return fmt.Errorf("%s: failed to prepare statement: %w", op, err)
	}

	_, err = stmt.ExecContext(ctx, UUID, expiresAt)
	if err != nil {
		return fmt.Errorf("%s: failed to insert session: %w", op, err)
	}

	return nil
}

func (s *Storage) GetSession(ctx context.Context, UUID string) (int, error) {
	const op = "storage.GetSession"

	stmt, err := s.db.Prepare(`
    	SELECT id
    	FROM sessions
    	WHERE uuid = ?`)
	if err != nil {
		return 0, fmt.Errorf("%s: failed to prepare statement: %w", op, err)
	}

	row := stmt.QueryRowContext(ctx, UUID)
	var id int

	err = row.Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("%s: session not found %w", op, err)
		}

		return 0, fmt.Errorf("%s: failed to fetch session: %w", op, err)
	}

	return id, nil
}

func (s *Storage) AddToCart(ctx context.Context, productID, quantity int, userID any) error {
	const op = "storage.AddToCart"

	stmt, err := s.db.Prepare(`
		INSERT INTO cart (product_id, quantity, user_id)
		VALUES(?, ?, ?)
		ON CONFLICT(user_id, product_id)
		DO UPDATE SET quantity = quantity + excluded.quantity;`)
	if err != nil {
		return fmt.Errorf("%s: failed to prepare statement: %w", op, err)
	}

	_, err = stmt.ExecContext(ctx, productID, quantity, userID)
	if err != nil {
		return fmt.Errorf("%s: failed to add to cart: %w", op, err)
	}

	return nil
}

func (s *Storage) GetCart(ctx context.Context, userID any) ([]models.CartItem, error) {
	const op = "storage.GetCart"

	stmt, err := s.db.Prepare(`
		SELECT p.id, p.name, p.description, p.price, c.quantity
		FROM cart AS c
		JOIN products AS p on p.id = c.product_id
		WHERE user_id = ?`)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to prepare statement: %w", op, err)
	}

	var cartItems []models.CartItem

	rows, err := stmt.QueryContext(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to fetch cart items: %w", op, err)
	}
	defer rows.Close()

	for rows.Next() {
		var c models.CartItem

		err = rows.Scan(
			&c.ProductID,
			&c.ProductName,
			&c.ProductDescription,
			&c.ProductPrice,
			&c.Quantity,
		)
		if err != nil {
			return nil, fmt.Errorf("%s: failed to fetch cart item: %w", op, err)
		}
		cartItems = append(cartItems, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan products: %w", err)
	}

	return cartItems, nil
}

func (s *Storage) GetCartCount(ctx context.Context, userID any) (int, error) {
	const op = "storage.GetCartCount"

	stmt, err := s.db.Prepare(`
		SELECT SUM(quantity)
		FROM cart
		WHERE user_id = ?`)
	if err != nil {
		return 0, fmt.Errorf("%s: failed to prepare statement: %w", op, err)
	}

	row := stmt.QueryRowContext(ctx, userID)
	var count int

	err = row.Scan(&count)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("%s: cart not found %w", op, err)
		}
		return 0, fmt.Errorf("%s: failed to fetch cart count: %w", op, err)
	}

	return count, nil
}

func (s *Storage) UpdateCartQuantity(ctx context.Context, productID, quantity int, userID any) error {
	const op = "storage.UpdateCartQuantity"

	stmt, err := s.db.Prepare(`
		UPDATE cart SET quantity = ?
		WHERE product_id = ? AND user_id = ?`)
	if err != nil {
		return fmt.Errorf("%s: failed to prepare statement: %w", op, err)
	}

	_, err = stmt.ExecContext(ctx, quantity, productID, userID)
	if err != nil {
		return fmt.Errorf("%s: failed to update cart quantity: %w", op, err)
	}

	return nil
}

func (s *Storage) RemoveFromCart(ctx context.Context, productID int, userID any) error {
	const op = "storage.RemoveFromCart"

	stmt, err := s.db.Prepare(`
		DELETE FROM cart WHERE product_id = ? AND user_id = ?`)
	if err != nil {
		return fmt.Errorf("%s: failed to prepare statement: %w", op, err)
	}

	_, err = stmt.ExecContext(ctx, productID, userID)
	if err != nil {
		return fmt.Errorf("%s: failed to remove from cart: %w", op, err)
	}

	return nil
}

func (s *Storage) MoveCart(ctx context.Context, newUserID int, oldUserID any) error {
	const op = "storage.MoveCart"

	fmt.Println("start moving")

	stmt, err := s.db.Prepare(`
		UPDATE cart SET user_id = ?
		WHERE user_id = ?`)
	if err != nil {
		return fmt.Errorf("%s: failed to prepare statement: %w", op, err)
	}

	fmt.Println("end moving")
	_, err = stmt.ExecContext(ctx, newUserID, oldUserID)
	if err != nil {
		return fmt.Errorf("%s: failed to move cart: %w", op, err)
	}

	return nil
}
