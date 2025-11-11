package services

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/h4ks-com/beapin/internal/models"
	"github.com/h4ks-com/beapin/internal/repository"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token expired")
)

type TokenClaims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

type TokenService struct {
	tokenRepo *repository.TokenRepository
	userRepo  *repository.UserRepository
	jwtSecret string
}

func NewTokenService(tokenRepo *repository.TokenRepository, userRepo *repository.UserRepository, jwtSecret string) *TokenService {
	return &TokenService{
		tokenRepo: tokenRepo,
		userRepo:  userRepo,
		jwtSecret: jwtSecret,
	}
}

func (s *TokenService) GenerateToken(username string, expiresIn time.Duration) (string, error) {
	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		return "", err
	}
	if user == nil {
		return "", ErrUserNotFound
	}

	claims := TokenClaims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "beapin",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", err
	}

	apiToken := &models.APIToken{
		UserID:    user.ID,
		Token:     tokenString,
		ExpiresAt: time.Now().Add(expiresIn),
	}

	err = s.tokenRepo.Create(apiToken)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func (s *TokenService) ValidateToken(tokenString string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(s.jwtSecret), nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	dbToken, err := s.tokenRepo.FindByToken(tokenString)
	if err != nil {
		return nil, err
	}
	if dbToken == nil {
		return nil, ErrInvalidToken
	}

	if dbToken.ExpiresAt.Before(time.Now()) {
		return nil, ErrExpiredToken
	}

	return claims, nil
}

func (s *TokenService) ListUserTokens(username string) ([]models.APIToken, error) {
	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	return s.tokenRepo.FindByUserID(user.ID)
}

func (s *TokenService) DeleteToken(tokenID uint, username string) error {
	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrUserNotFound
	}

	return s.tokenRepo.Delete(tokenID, user.ID)
}
