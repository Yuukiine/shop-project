package cart

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"go.uber.org/zap"

	"shop/internal/domain/models"
	"shop/lib/jwt"
)

const (
	shippingPrice = 4.99
	taxPercent    = 0.21
)

type Storage interface {
	User(ctx context.Context, email string) (models.User, error)
	GetCart(ctx context.Context, userID int) ([]models.CartItem, error)
	UpdateCartQuantity(ctx context.Context, productID, userID, quantity int) error
	RemoveFromCart(ctx context.Context, productID, userID int) error
}

type Handler struct {
	logger  *zap.Logger
	tmpl    *template.Template
	storage Storage
}

func NewCartHandler(storage Storage, logger *zap.Logger) *Handler {
	tmpl, err := template.ParseFiles("./html-templates/shopping_cart_page.html")
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
	Title      string            `json:"title"`
	User       string            `json:"user"`
	Email      string            `json:"email"`
	TotalItems int               `json:"totalItems"`
	Subtotal   float64           `json:"subtotal"`
	Shipping   float64           `json:"shipping"`
	Tax        float64           `json:"tax"`
	Total      float64           `json:"total"`
	CartCount  int               `json:"cartCount"`
	Success    bool              `json:"success"`
	Error      string            `json:"error"`
	CartItems  []models.CartItem `json:"cartItems"`
}

type Sum struct {
	CartItems  []models.CartItem
	CartCount  int
	TotalItems int
	Subtotal   float64
	Shipping   float64
	Tax        float64
	Total      float64
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	data := PageData{
		Title: "Cart",
	}

	cookie, err := r.Cookie("auth_token")
	if err == nil {
		email, err := jwt.GetEmailFromToken(cookie.Value)
		if err != nil {
			h.logger.Error("failed to parse token", zap.Error(err))
		}
		data.User = "true"
		data.Email = email
	}

	var cart []models.CartItem
	var userID int
	user, err := h.storage.User(r.Context(), data.Email)
	if err != nil {
		h.logger.Error("failed to fetch user", zap.Error(err))
	} else {
		userID = user.ID
	}

	cart, err = h.storage.GetCart(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to fetch cart", zap.Error(err))
		h.ServeHTTPWithError(w, "Failed to load your cart. Please try again later")
	}

	summary := calculateCartSum(cart)

	data.CartItems = cart
	data.TotalItems = summary.TotalItems
	data.Subtotal = summary.Subtotal
	data.Shipping = summary.Shipping
	data.Tax = summary.Tax
	data.Total = summary.Total
	data.CartCount = summary.CartCount

	if err := h.tmpl.Execute(w, data); err != nil {
		h.logger.Error("failed to execute home template", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) UpdateHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		h.logger.Error("failed to parse form", zap.Error(err))
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	var (
		userID int
		user   models.User
	)

	productStr := r.FormValue("product_id")
	productID, err := strconv.Atoi(productStr)
	if err != nil {
		h.logger.Error("failed to parse product id", zap.Error(err))
	}

	quantityStr := r.FormValue("quantity")
	quantity, err := strconv.Atoi(quantityStr)
	if err != nil {
		h.logger.Error("invalid quantity", zap.Error(err))
		quantity = 1
	}

	if quantity < 0 {
		h.SendJSONError(w, "Quantity cannot be negative", http.StatusBadRequest)
		return
	}

	cookie, err := r.Cookie("auth_token")
	if err == nil {
		email, err := jwt.GetEmailFromToken(cookie.Value)
		if err != nil {
			h.logger.Error("failed to parse token", zap.Error(err))
			return
		}

		user, err = h.storage.User(r.Context(), email)
		if err != nil {
			h.logger.Error("failed to fetch user", zap.Error(err))
			return
		}
		userID = user.ID
	}

	err = h.storage.UpdateCartQuantity(r.Context(), productID, userID, quantity)
	if err != nil {
		h.logger.Error("failed to update cart quantity", zap.Error(err))
		h.SendJSONError(w, "Failed to update cart. Please try again.", http.StatusInternalServerError)
		return
	}

	time.Sleep(50 * time.Millisecond)

	cart, err := h.storage.GetCart(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to fetch cart", zap.Error(err))
		h.SendJSONError(w, "Failed to load updated cart", http.StatusInternalServerError)
		return
	}

	sum := calculateCartSum(cart)

	data := PageData{
		Success:    true,
		Title:      "Cart",
		TotalItems: sum.TotalItems,
		Subtotal:   sum.Subtotal,
		Shipping:   sum.Shipping,
		Tax:        sum.Tax,
		CartItems:  cart,
		CartCount:  sum.CartCount,
		Total:      sum.Total,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode JSON response", zap.Error(err))
	}
}

func (h *Handler) RemoveHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		h.logger.Error("failed to parse form", zap.Error(err))
	}

	var (
		userID int
		user   models.User
	)

	productStr := r.FormValue("product_id")
	productID, err := strconv.Atoi(productStr)
	if err != nil {
		h.logger.Error("failed to parse product id", zap.Error(err))
	}

	cookie, err := r.Cookie("auth_token")
	if err == nil {
		email, err := jwt.GetEmailFromToken(cookie.Value)
		if err != nil {
			h.logger.Error("failed to parse token", zap.Error(err))
			return
		}

		user, err = h.storage.User(r.Context(), email)
		if err != nil {
			h.logger.Error("failed to fetch user", zap.Error(err))
			return
		}
		userID = user.ID
	}

	err = h.storage.RemoveFromCart(r.Context(), productID, userID)
	if err != nil {
		h.logger.Error("failed to remove from cart", zap.Error(err))
		h.SendJSONError(w, "Failed to remove fro cart. Please try again later", http.StatusInternalServerError)
		return
	}

	time.Sleep(50 * time.Millisecond)

	cart, err := h.storage.GetCart(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to fetch cart", zap.Error(err))
		h.SendJSONError(w, "Failed to load updated cart", http.StatusInternalServerError)
		return
	}

	sum := calculateCartSum(cart)

	data := PageData{
		Success:    true,
		Title:      "Cart",
		TotalItems: sum.TotalItems,
		Subtotal:   sum.Subtotal,
		Shipping:   sum.Shipping,
		Tax:        sum.Tax,
		CartItems:  cart,
		CartCount:  sum.CartCount,
		Total:      sum.Total,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode JSON response", zap.Error(err))
	}
}

func calculateCartSum(cart []models.CartItem) Sum {
	var sum Sum

	for _, v := range cart {
		sum.Subtotal += v.ProductPrice * float64(v.Quantity)
		sum.CartCount += v.Quantity
	}

	sum.TotalItems = sum.CartCount
	sum.Tax = sum.Subtotal * taxPercent
	sum.Shipping = shippingPrice
	sum.Total = sum.Subtotal + sum.Tax + sum.Shipping

	return sum
}

func (h *Handler) ServeHTTPWithError(w http.ResponseWriter, errorMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)

	data := PageData{
		Title: "Cart",
		Error: errorMsg,
	}

	if err := h.tmpl.Execute(w, data); err != nil {
		h.logger.Error("failed to execute cart template with error", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (h *Handler) SendJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := PageData{
		Success: false,
		Error:   message,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("failed to encode error response", zap.Error(err))
	}
}
