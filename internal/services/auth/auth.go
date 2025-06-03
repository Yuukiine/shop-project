package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"shop/internal/domain/models"
	"shop/internal/storage"
	"shop/lib/jwt"
)

type Auth struct {
	log         *zap.Logger
	usrSaver    UserSaver
	usrProvider UserProvider
	appProvider AppProvider
	tokenTTL    time.Duration
}

type UserSaver interface {
	SaveUser(
		ctx context.Context,
		email string,
		passHash []byte,
	) (uid int64, err error)
}

type UserProvider interface {
	User(ctx context.Context, email string) (models.User, error)
	IsAdmin(ctx context.Context, userID int64) (bool, error)
}

type AppProvider interface {
	App(ctx context.Context, appID int) (models.App, error)
}

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidAppID       = errors.New("invalid app id")
	ErrUserExists         = errors.New("user already exists")
	ErrUserNotFound       = errors.New("user not found")
)

// New returns a new instance of the Auth service
func New(
	log *zap.Logger,
	usrSaver UserSaver,
	usrProvider UserProvider,
	appProvider AppProvider,
	tokenTTL time.Duration,
) *Auth {
	return &Auth{
		usrSaver:    usrSaver,
		usrProvider: usrProvider,
		log:         log,
		appProvider: appProvider,
		tokenTTL:    tokenTTL,
	}
}

// Login checks if user with given credentials exists in the system and returns access token
//
// If user exists, but password is incorrect, returns error
// If user doesn't exist, returns error.
func (a *Auth) Login(
	ctx context.Context,
	email string,
	password string,
	appID int,
) (string, error) {
	const op = "auth.Login"

	log := a.log.With(
		zap.String("op", op),
		zap.String("username", email),
	)

	log.Info("attempting to login user")

	user, err := a.usrProvider.User(ctx, email)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			log.Warn("user not found: " + err.Error())

			return "", fmt.Errorf("%s, %w", op, ErrUserNotFound)
		}
		log.Error("failed to get user: " + err.Error())

		return "", fmt.Errorf("%s, %w", op, err)
	}

	if err := bcrypt.CompareHashAndPassword(user.PassHash, []byte(password)); err != nil {
		log.Error("invalid credentials: " + err.Error())

		return "", ErrInvalidCredentials
	}

	app, err := a.appProvider.App(ctx, appID)
	if err != nil {
		return "", fmt.Errorf("%s, %w", op, err)
	}

	log.Info("user logged in successfully")

	token, err := jwt.NewToken(user, app, a.tokenTTL)
	if err != nil {
		log.Error("failed to generate token: " + err.Error())

		return "", fmt.Errorf("%s, %w", op, err)
	}

	return token, nil
}

// RegisterNewUser registers new user in the system and returns user ID.
// If user with given username already exists, returns error
func (a *Auth) RegisterNewUser(ctx context.Context, email string, pass string) (int64, error) {
	const op = "auth.registerNewUser"

	log := a.log.With(
		zap.String("op", op),
		zap.String("email", email),
	)

	log.Info("registering new user")

	passHash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.MinCost)
	if err != nil {
		log.Error("failed to hash password: ", zap.Error(err))

		return 0, fmt.Errorf("%s, %w", op, err)
	}

	id, err := a.usrSaver.SaveUser(ctx, email, passHash)
	if err != nil {
		if errors.Is(err, storage.ErrUserExists) {
			log.Error("user already exists: ", zap.Error(err))

			return 0, fmt.Errorf("%s, %w", op, ErrUserExists)
		}

		log.Error("failed to save user: ", zap.Error(err))

		return 0, fmt.Errorf("%s, %w", op, err)
	}

	log.Info("user registered")

	return id, nil
}

// IsAdmin checks if user is an admin
func (a *Auth) IsAdmin(ctx context.Context, userID int64) (bool, error) {
	const op = "auth.isAdmin"

	log := a.log.With(
		zap.String("op", op),
		zap.Int64("userID", userID),
	)

	log.Info("checking if user is admin")

	isAdmin, err := a.usrProvider.IsAdmin(ctx, userID)
	if err != nil {
		if errors.Is(err, storage.ErrAppNotFound) {
			log.Warn("app not found: " + err.Error())

			return false, fmt.Errorf("%s, %w", op, ErrInvalidAppID)
		}

		return false, fmt.Errorf("%s, %w", op, err)
	}

	log.Info("checking if user is admin", zap.Bool("isAdmin", isAdmin))

	return isAdmin, nil
}
