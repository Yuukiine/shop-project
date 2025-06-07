package home

import (
	"context"
	"html/template"
	"net/http"

	"go.uber.org/zap"

	"shop/internal/domain/models"
	"shop/internal/http-server/cookies"
	"shop/internal/random"
	"shop/lib/jwt"
)

type Storage interface {
	GetProduct(ctx context.Context, id int) (models.Product, error)
	CreateSession(ctx context.Context, UUID string) error
	GetCartCount(ctx context.Context, userID int) (int, error)
	User(ctx context.Context, email string) (models.User, error)
}

type Handler struct {
	storage Storage
	logger  *zap.Logger
	tmpl    *template.Template
}

func NewHomeHandler(storage Storage, logger *zap.Logger) *Handler {
	tmpl, err := template.ParseFiles("./html-templates/home_page.html")
	if err != nil {
		logger.Fatal("failed to parse home template", zap.Error(err))
	}

	return &Handler{
		storage: storage,
		logger:  logger,
		tmpl:    tmpl,
	}
}

type PageData struct {
	User      string
	Email     string
	Title     string
	CartCount int
	Products  []models.Product
	Success   bool `json:"success"`
	Error     string
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	var (
		userID int
		user   models.User
	)

	data := PageData{
		Title: "Welcome to Our Shop",
	}

	cookie, err := r.Cookie("auth_token")
	if err == nil {
		email, err := jwt.GetEmailFromToken(cookie.Value)
		if err != nil {
			h.logger.Error("failed to parse token", zap.Error(err))
		}

		data.User = "true"
		data.Email = email

		user, err = h.storage.User(r.Context(), email)
		if err != nil {
			h.logger.Error("failed to fetch user", zap.Error(err))
			return
		}
		userID = user.ID
	} else {
		UUID := cookies.SetSessionCookie(w)
		err = h.storage.CreateSession(r.Context(), UUID)
		if err != nil {
			h.logger.Error("failed to create session", zap.Error(err))
		}
	}

	cartCount, err := h.storage.GetCartCount(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to fetch cart count", zap.Error(err))
	}
	data.CartCount = cartCount

	products := make([]models.Product, 9)
	for i := range 9 {
		product, err := h.storage.GetProduct(r.Context(), random.RandomProductID(100))
		if err != nil {
			h.logger.Error("failed to fetch products", zap.Error(err))
			data.Error = "Unable to load products at this time. Please try again later."
		}
		products[i] = product
	}

	data.Products = products
	h.logger.Info("loaded products for home page", zap.Int("count", len(products)))

	if err := h.tmpl.Execute(w, data); err != nil {
		h.logger.Error("failed to execute home template", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}
