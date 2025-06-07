package jwt

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"shop/internal/domain/models"
)

func NewToken(user models.User, app models.App, duration time.Duration) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)

	claims := token.Claims.(jwt.MapClaims)
	claims["uid"] = user.ID
	claims["email"] = user.Email
	claims["exp"] = time.Now().Add(duration).Unix()
	claims["app_id"] = app.ID

	tokenString, err := token.SignedString([]byte(app.Secret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func GetEmailFromToken(tokenString string) (string, error) {
	const op = "jwt.ParseTokenForEmail"

	secretKey := []byte("test-secret")
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%s: unexpected signing method: %v", op, token.Header["alg"])
		}
		return secretKey, nil
	})

	if err != nil {
		return "", err
	}

	var email string

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		email = claims["email"].(string)
	} else {
		return "", fmt.Errorf("invalid token")
	}

	return email, nil
}
