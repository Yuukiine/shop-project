package products

import (
	"context"
	"html/template"
	"net/http"
	"strconv"

	"go.uber.org/zap"

	"shop/internal/domain/models"
	"shop/lib/jwt"
)

const (
	productsPerPage = 15
	defaultPage     = 1
)

type Storage interface {
	GetProducts(ctx context.Context, limit, offset int) ([]models.Product, error)
	TotalProducts(ctx context.Context) (int, error)
}

type Handler struct {
	logger  *zap.Logger
	tmpl    *template.Template
	storage Storage
}

func NewProductsHandler(storage Storage, logger *zap.Logger) *Handler {
	tmpl, err := template.ParseFiles("./html-templates/products_page.html")
	if err != nil {
		logger.Fatal("failed to parse products template", zap.Error(err))
	}

	return &Handler{
		logger:  logger,
		tmpl:    tmpl,
		storage: storage,
	}
}

type PageData struct {
	Title         string
	User          string
	Email         string
	Products      []models.Product
	Error         string
	CurrentPage   int
	TotalPages    int
	TotalProducts int
	StartResult   int
	EndResult     int
	PrevPage      int
	NextPage      int
	PageNumbers   []int
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	pageStr := r.URL.Query().Get("page")
	page := defaultPage

	if pageStr != "" {
		p, err := strconv.Atoi(pageStr)
		if err != nil {
			h.logger.Warn("failed to parse page number", zap.String("page", pageStr), zap.Error(err))
		}

		if p > 0 {
			page = p
		}
	}

	data := PageData{
		Title:       "Look at our products!",
		CurrentPage: page,
	}

	products, err := h.storage.GetProducts(r.Context(), productsPerPage, (page-1)*productsPerPage)
	if err != nil {
		h.logger.Warn("failed to fetch products", zap.Error(err))
		data.Error = "Unable to load products at this time. Please try again later."
		return
	}
	data.Products = products

	totalProducts, err := h.storage.TotalProducts(r.Context())
	if err != nil {
		h.logger.Warn("failed to fetch total products", zap.Error(err))
	}
	data.TotalProducts = totalProducts

	totalPages := totalProducts/productsPerPage + 1
	data.TotalPages = totalPages

	startResult := (page-1)*productsPerPage + 1
	endResult := startResult + len(products) - 1
	if len(products) == 0 {
		startResult = 0
		endResult = 0
	}
	data.StartResult = startResult
	data.EndResult = endResult

	prevPage := page - 1
	if prevPage < 1 {
		prevPage = 1
	}
	data.PrevPage = prevPage

	nextPage := page + 1
	if nextPage > totalPages {
		nextPage = totalPages
	}
	data.NextPage = nextPage

	if page > totalPages && totalPages > 0 {
		http.Redirect(w, r, "/products?page=1", http.StatusFound)
		return
	}

	cookie, err := r.Cookie("auth_token")
	if err == nil {
		email, err := jwt.ParseTokenForEmail(h.logger, cookie.Value)
		if err != nil {
			h.logger.Error("failed to parse token", zap.Error(err))
		}
		data.User = "true"
		data.Email = email
	}

	data.PageNumbers = generatePageNumbers(page, totalPages)

	if err := h.tmpl.Execute(w, data); err != nil {
		h.logger.Error("failed to execute home template", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func generatePageNumbers(currentPage, totalPages int) []int {
	var pages []int

	start := currentPage - 2
	if start < 1 {
		start = 1
	}

	end := currentPage + 2
	if end > totalPages {
		end = totalPages
	}

	for i := start; i <= end; i++ {
		pages = append(pages, i)
	}

	return pages
}
