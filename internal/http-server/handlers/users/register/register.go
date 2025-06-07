package register

import (
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"time"

	ssov1 "github.com/Yuukiine/protos/gen/go/sso"
	"go.uber.org/zap"

	"shop/internal/grpc/auth"
)

type Handler struct {
	AuthClient ssov1.AuthClient
	logger     *zap.Logger
	tmpl       *template.Template
}

func NewRegisterHandler(authClient ssov1.AuthClient, logger *zap.Logger) *Handler {
	tmpl, err := template.ParseFiles("./html-templates/register_page.html")
	if err != nil {
		logger.Fatal("failed to parse home template", zap.Error(err))
	}

	return &Handler{
		logger:     logger,
		tmpl:       tmpl,
		AuthClient: authClient,
	}
}

func (h *Handler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.logger.Error("failed to parse register form", zap.Error(err))
		http.Redirect(w, r, "/register?error=Invalid+form+data", http.StatusSeeOther)
		return
	}
	var req struct {
		Email           string `json:"email"`
		Password        string `json:"password"`
		ConfirmPassword string `json:"confirmPassword"`
	}

	email := req.Email
	password := req.Password
	confirm := req.ConfirmPassword

	if password != confirm {
		h.logger.Error("login failed",
			zap.String("error", "mismatched passwords"))
		h.ServeHTTPWithError(w, "Passwords don't match.", http.StatusBadRequest)
		return
	}

	_, err := h.AuthClient.Register(r.Context(), &ssov1.RegisterRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		h.logger.Error("failed to register user", zap.Error(err))
		if errors.Is(err, auth.ErrExists) {
			h.ServeHTTPWithError(w, "User with this email already exists...", http.StatusConflict)
			return
		}
		h.ServeHTTPWithError(w, "Internal error. Please try again later.", http.StatusInternalServerError)
		return
	}

	h.logger.Info("user registered successfully",
		zap.String("email", email))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Account created successfully! Redirecting to login...",
	})
	time.Sleep(1 * time.Second)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := h.tmpl.Execute(w, ""); err != nil {
		h.logger.Error("failed to execute home template", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) ServeHTTPWithError(w http.ResponseWriter, errorMsg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": errorMsg,
	})
}
