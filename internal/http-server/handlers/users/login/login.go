package login

import (
	"context"
	"errors"
	"html/template"
	"net/http"
	"time"

	ssov1 "github.com/Yuukiine/protos/gen/go/sso"
	"go.uber.org/zap"

	"shop/internal/domain/models"
	"shop/internal/grpc/auth"
	"shop/internal/http-server/cookies"
)

type Storage interface {
	User(ctx context.Context, email string) (models.User, error)
	MoveCart(ctx context.Context, newUserID int, oldUserID any) error
	GetCartCount(ctx context.Context, userID any) (int, error)
}

type Handler struct {
	storage    Storage
	AuthClient ssov1.AuthClient
	logger     *zap.Logger
	tmpl       *template.Template
}

func NewLoginHandler(storage Storage, authClient ssov1.AuthClient, logger *zap.Logger) *Handler {
	tmpl, err := template.ParseFiles("./html-templates/login_page.html")
	if err != nil {
		logger.Fatal("failed to parse home template", zap.Error(err))
	}

	return &Handler{
		storage:    storage,
		logger:     logger,
		tmpl:       tmpl,
		AuthClient: authClient,
	}
}

type PageData struct {
	Title   string
	Error   string
	Success string
	Email   string
}

func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.logger.Error("failed to parse login form", zap.Error(err))
		http.Redirect(w, r, "/login?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	var (
		userID int
		user   models.User
		sessID any
	)

	cookie, err := r.Cookie("session_id")
	if err == nil {
		sessID = cookie.Value
	}

	email := r.FormValue("email")
	password := r.FormValue("password")

	if email == "" || password == "" {
		h.logger.Warn("login attempt with missing credentials")
		return
	}
	logResp, err := h.AuthClient.Login(r.Context(), &ssov1.LoginRequest{
		Email:    email,
		Password: password,
		AppId:    1,
	})
	if err != nil {
		h.logger.Error("login failed",
			zap.String("email", email),
			zap.Error(err))
		if errors.Is(err, auth.ErrInvalidArgument) {
			h.ServeHTTPWithError(w, "Invalid credentials. Please try again", email)
			return
		}
		if errors.Is(err, auth.ErrNotFound) {
			h.ServeHTTPWithError(w, "User with this email doesn't exist. Please complete registration, or try another credentials", email)
			return
		}

		h.ServeHTTPWithError(w, "Internal error. Please try again later", email)
		return
	}

	cookies.ClearSessionCookie(w)
	cookies.SetAuthCookie(w, logResp.Token, time.Now().Add(72*time.Hour))

	h.logger.Info("user logged in successfully",
		zap.String("email", email))

	cartCount, err := h.storage.GetCartCount(r.Context(), sessID)
	if err != nil {
		h.logger.Error("failed to fetch cart count", zap.Error(err))
	}

	if cartCount > 0 {
		user, err = h.storage.User(r.Context(), email)
		if err != nil {
			h.logger.Error("failed to fetch user", zap.Error(err))
			return
		}
		userID = user.ID

		err = h.storage.MoveCart(r.Context(), userID, sessID)
		if err != nil {
			h.logger.Error("failed to move cart", zap.Error(err))
		}
	}

	h.logger.Info("cart moved successfully")

	redirectTo := r.URL.Query().Get("redirect")
	if redirectTo == "" {
		redirectTo = "/"
	}

	http.Redirect(w, r, redirectTo, http.StatusSeeOther)
}

func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("auth_token")
	if err == nil && cookie.Value != "" {
		h.logger.Error("failed to logout on auth service", zap.Error(err))
	}

	cookies.ClearAuthCookie(w)

	h.logger.Info("user logged out")

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	data := PageData{
		Title: "Login to Your Account",
		Error: "",
	}

	if err := h.tmpl.Execute(w, data); err != nil {
		h.logger.Error("failed to execute home template", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) ServeHTTPWithError(w http.ResponseWriter, errorMsg, email string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)

	data := PageData{
		Title: "Login to Your Account",
		Error: errorMsg,
		Email: email,
	}

	if err := h.tmpl.Execute(w, data); err != nil {
		h.logger.Error("failed to execute login template with error", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
