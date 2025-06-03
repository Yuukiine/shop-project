package login

import (
	"context"
	"html/template"
	"net/http"
	"time"

	ssov1 "github.com/Yuukiine/protos/gen/go/sso"
	"go.uber.org/zap"

	"shop/internal/domain/models"
)

type Handler struct {
	storage    Storage
	AuthClient ssov1.AuthClient
	logger     *zap.Logger
	tmpl       *template.Template
}

type Storage interface {
	User(ctx context.Context, email string) (models.User, error)
}

func NewLoginHandler(authClient ssov1.AuthClient, storage Storage, logger *zap.Logger) *Handler {
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
	time.Sleep(2 * time.Second)
	if err := r.ParseForm(); err != nil {
		h.logger.Error("failed to parse login form", zap.Error(err))
		http.Redirect(w, r, "/login?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")

	if email == "" || password == "" {
		h.logger.Warn("login attempt with missing credentials")
		return
	}
	loginResp, err := h.AuthClient.Login(r.Context(), &ssov1.LoginRequest{
		Email:    email,
		Password: password,
		AppId:    1,
	})
	if err != nil {
		h.logger.Error("login failed",
			zap.String("email", email),
			zap.Error(err))
		return
	}

	h.setAuthCookie(w, loginResp.Token, time.Now().Add(1*time.Hour))

	h.logger.Info("user logged in successfully",
		zap.String("email", email))

	redirectTo := r.URL.Query().Get("redirect")
	if redirectTo == "" {
		redirectTo = "/"
	}
	http.Redirect(w, r, redirectTo, http.StatusSeeOther)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	data := PageData{
		Title: "Login to Your Account",
	}

	if msg := r.URL.Query().Get("error"); msg != "" {
		data.Error = msg
	}
	if msg := r.URL.Query().Get("success"); msg != "" {
		data.Success = msg
	}

	if err := h.tmpl.Execute(w, data); err != nil {
		h.logger.Error("failed to execute home template", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) setAuthCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	cookie := &http.Cookie{
		Name:     "auth_token",
		Value:    token,
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	}
	http.SetCookie(w, cookie)
}

func (h *Handler) clearAuthCookie(w http.ResponseWriter) {
	cookie := &http.Cookie{
		Name:     "auth_token",
		Value:    "",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   -1,
	}
	http.SetCookie(w, cookie)
}
