package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	ssov1 "github.com/Yuukiine/protos/gen/go/sso"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"shop/internal/app"
	"shop/internal/config"
	"shop/internal/http-server/handlers/cart"
	"shop/internal/http-server/handlers/home"
	"shop/internal/http-server/handlers/products"
	"shop/internal/http-server/handlers/users/login"
	"shop/internal/http-server/handlers/users/register"
	zapper "shop/internal/logger"
	mwLogger "shop/internal/logger/middleware"
	"shop/internal/storage/sqlite"
	kafka2 "shop/kafka"
)

func main() {
	cfg := config.MustLoad()

	logger := zapper.NewLogger(cfg.Env)

	logger.Info("starting application", zap.Any("cfg", cfg))

	application := app.New(logger, cfg.GRPC.Port, cfg.StoragePath, cfg.TokenTTL)

	kafka := kafka2.New(logger)

	go func() {
		application.GRPCServ.MustRun()
	}()

	time.Sleep(2 * time.Second)

	g, err := setupGRPCClient(cfg, logger)
	if err != nil {
		logger.Fatal("failed to setup gRPC client", zap.Error(err))
	}
	var authClient = ssov1.NewAuthClient(g)

	defer g.Close()

	storage, err := sqlite.New(cfg.StoragePath)
	if err != nil {
		logger.Fatal("failed to init storage",
			zap.Time("time:", time.Now()),
			zap.String("error:", fmt.Sprintf("failed to init sqlite storage: %v", err)))
	}

	homeHandler := home.NewHomeHandler(storage, logger)
	loginHandler := login.NewLoginHandler(storage, authClient, logger)
	registerHandler := register.NewRegisterHandler(kafka, authClient, logger)
	productsHandler := products.NewProductsHandler(storage, logger)
	cartHandler := cart.NewCartHandler(storage, logger)

	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(middleware.Logger)
	router.Use(mwLogger.New(logger))
	router.Use(middleware.Recoverer)
	router.Use(middleware.URLFormat)

	logger.Info("starting server", zap.String("address", cfg.Address))
	router.Get("/", homeHandler.ServeHTTP)
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
	})

	go router.Route("/login", func(r chi.Router) {
		r.Get("/", loginHandler.ServeHTTP)
		r.Post("/", loginHandler.HandleLogin)
	})
	router.Route("/register", func(r chi.Router) {
		r.Get("/", registerHandler.ServeHTTP)
		r.Post("/", registerHandler.HandleRegister)
	})
	router.Get("/logout", loginHandler.HandleLogout)

	go router.Route("/products", func(r chi.Router) {
		r.Get("/", productsHandler.ServeHTTP)
	})

	go router.Route("/cart", func(r chi.Router) {
		r.Get("/", cartHandler.ServeHTTP)
		r.Post("/add", productsHandler.AddToCart)
		r.Post("/update", cartHandler.UpdateHandler)
		r.Post("/remove", cartHandler.RemoveHandler)
	})

	srv := &http.Server{
		Addr:    cfg.Address,
		Handler: router,
	}

	if err := srv.ListenAndServe(); err != nil {
		logger.Fatal("failed to start server", zap.Error(err))
	}
}

func setupGRPCClient(cfg *config.Config, logger *zap.Logger) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	address := grpcAddress(cfg)
	logger.Info("attempting gRPC connection", zap.String("address", address))

	conn, err := grpc.DialContext(ctx, address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gRPC server: %w", err)
	}

	return conn, nil
}

func grpcAddress(cfg *config.Config) string {
	return net.JoinHostPort("localhost", strconv.Itoa(cfg.GRPC.Port))
}
