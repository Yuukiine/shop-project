package home

import (
	"context"
	"html/template"
	"net/http"

	"go.uber.org/zap"

	"shop/internal/domain/models"
)

type Storage interface {
	GetProducts(ctx context.Context, limit, offset int) ([]models.Product, error)
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
	Title    string
	Products []models.Product
	Error    string
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	data := PageData{
		Title: "Welcome to Our Shop",
	}

	products, err := h.storage.GetProducts(r.Context(), 10, 0)
	if err != nil {
		h.logger.Error("failed to fetch products", zap.Error(err))
		data.Error = "Unable to load products at this time. Please try again later."
	} else {
		data.Products = products
		h.logger.Info("loaded products for home page", zap.Int("count", len(products)))
	}

	if err := h.tmpl.Execute(w, data); err != nil {
		h.logger.Error("failed to execute home template", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}
