package auth

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/h4ks-com/beapin/internal/config"
	"github.com/logto-io/go/v2/client"
)

type LogtoHandler struct {
	config *config.LogtoConfig
}

func NewLogtoHandler(cfg *config.LogtoConfig) *LogtoHandler {
	return &LogtoHandler{config: cfg}
}

func (h *LogtoHandler) CreateLogtoClient(ctx *gin.Context) *client.LogtoClient {
	session := sessions.Default(ctx)
	logtoConfig := &client.LogtoConfig{
		Endpoint:  h.config.Endpoint,
		AppId:     h.config.AppID,
		AppSecret: h.config.AppSecret,
	}
	return client.NewLogtoClient(logtoConfig, NewSessionStorage(session))
}

func (h *LogtoHandler) Login(ctx *gin.Context) {
	logtoClient := h.CreateLogtoClient(ctx)

	signInUri, err := logtoClient.SignIn(&client.SignInOptions{
		RedirectUri: h.config.RedirectURI,
	})
	if err != nil {
		ctx.String(http.StatusInternalServerError, fmt.Sprintf("Failed to initiate sign-in: %v", err))
		return
	}

	ctx.Redirect(http.StatusTemporaryRedirect, signInUri)
}

func (h *LogtoHandler) Callback(ctx *gin.Context) {
	log.Println("[LogtoHandler] Callback started")
	logtoClient := h.CreateLogtoClient(ctx)

	err := logtoClient.HandleSignInCallback(ctx.Request)
	if err != nil {
		log.Printf("[LogtoHandler] Callback error: %v", err)
		ctx.String(http.StatusInternalServerError, fmt.Sprintf("Failed to handle callback: %v", err))
		return
	}

	log.Printf("[LogtoHandler] Callback successful, IsAuthenticated: %v", logtoClient.IsAuthenticated())
	ctx.Redirect(http.StatusFound, "/")
}

func (h *LogtoHandler) Logout(ctx *gin.Context) {
	logtoClient := h.CreateLogtoClient(ctx)

	signOutUri, err := logtoClient.SignOut(h.config.PostLogoutURI)
	if err != nil {
		ctx.String(http.StatusInternalServerError, fmt.Sprintf("Failed to initiate sign-out: %v", err))
		return
	}

	ctx.Redirect(http.StatusTemporaryRedirect, signOutUri)
}

func (h *LogtoHandler) RequireAuth() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		log.Printf("[LogtoHandler] RequireAuth for path: %s", ctx.Request.URL.Path)
		logtoClient := h.CreateLogtoClient(ctx)

		isAuth := logtoClient.IsAuthenticated()
		log.Printf("[LogtoHandler] IsAuthenticated: %v", isAuth)

		if !isAuth {
			log.Println("[LogtoHandler] Not authenticated, redirecting to login")
			ctx.Redirect(http.StatusFound, "/auth/login")
			ctx.Abort()
			return
		}

		idTokenClaims, err := logtoClient.GetIdTokenClaims()
		if err != nil {
			log.Printf("[LogtoHandler] Failed to get ID token claims: %v", err)
			ctx.Redirect(http.StatusFound, "/auth/login")
			ctx.Abort()
			return
		}

		log.Printf("[LogtoHandler] Successfully authenticated user: %s", idTokenClaims.Sub)
		ctx.Set("username", idTokenClaims.Sub)
		ctx.Next()
	}
}

func (h *LogtoHandler) GetCurrentUser(ctx *gin.Context) (string, bool) {
	userID, exists := ctx.Get("username")
	if !exists {
		return "", false
	}
	return userID.(string), true
}
