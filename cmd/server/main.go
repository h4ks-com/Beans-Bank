package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/h4ks-com/beapin/internal/auth"
	"github.com/h4ks-com/beapin/internal/config"
	"github.com/h4ks-com/beapin/internal/database"
	"github.com/h4ks-com/beapin/internal/handlers"
	"github.com/h4ks-com/beapin/internal/middleware"
	"github.com/h4ks-com/beapin/internal/repository"
	"github.com/h4ks-com/beapin/internal/services"

	_ "github.com/h4ks-com/beapin/docs"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title           Beapin API
// @version         1.0
// @description     Bean currency management system for h4ks.com
// @BasePath        /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load configuration:", err)
	}

	gin.SetMode(cfg.GinMode)

	db, err := database.Connect(cfg.Database.URL)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	if err := database.Migrate(db); err != nil {
		log.Fatal("Failed to migrate database:", err)
	}

	userRepo := repository.NewUserRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)
	tokenRepo := repository.NewTokenRepository(db)

	walletService := services.NewWalletService(userRepo, transactionRepo)
	transferService := services.NewTransferService(userRepo, transactionRepo, db)
	tokenService := services.NewTokenService(tokenRepo, userRepo, cfg.JWT.Secret)

	authMiddleware := middleware.NewAuthMiddleware(tokenService, cfg.TestMode)
	adminMiddleware := middleware.NewAdminMiddleware(cfg.AdminUsers)

	walletHandler := handlers.NewWalletHandler(walletService)
	transferHandler := handlers.NewTransferHandler(transferService)
	tokenHandler := handlers.NewTokenHandler(tokenService)
	adminHandler := handlers.NewAdminHandler(userRepo, transactionRepo, walletService)
	publicHandler := handlers.NewPublicHandler(walletService)

	router := gin.Default()

	store := cookie.NewStore([]byte(cfg.Session.Secret))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 7 days
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
	})
	router.Use(sessions.Sessions("beapin_session", store))

	logtoHandler := auth.NewLogtoHandler(&cfg.Logto)

	router.LoadHTMLGlob("web/templates/*")

	authRoutes := router.Group("/auth")
	{
		authRoutes.GET("/login", logtoHandler.Login)
		authRoutes.GET("/callback", logtoHandler.Callback)
		authRoutes.GET("/logout", logtoHandler.Logout)
	}

	router.GET("/", func(c *gin.Context) {
		log.Println("[Homepage] Request received")
		total, _ := walletService.GetTotalBeans()

		isAuthenticated := false
		username := ""
		if !cfg.TestMode {
			log.Println("[Homepage] Checking authentication")
			logtoClient := logtoHandler.CreateLogtoClient(c)
			isAuthenticated = logtoClient.IsAuthenticated()
			log.Printf("[Homepage] IsAuthenticated: %v", isAuthenticated)
			if isAuthenticated {
				if claims, err := logtoClient.GetIdTokenClaims(); err == nil {
					username = claims.Sub
					log.Printf("[Homepage] User: %s", username)
				}
			}
		}

		c.HTML(200, "index.html", gin.H{
			"TotalBeans":      total,
			"IsAuthenticated": isAuthenticated,
			"Username":        username,
			"TestMode":        cfg.TestMode,
		})
	})

	router.GET("/transfer/:from/:to/:amount", func(c *gin.Context) {
		from := c.Param("from")
		to := c.Param("to")
		amount := c.Param("amount")

		c.HTML(200, "transfer.html", gin.H{
			"FromUser":  from,
			"ToUser":    to,
			"Amount":    amount,
			"NeedsAuth": !cfg.TestMode,
		})
	})

	router.POST("/transfer/:from/:to/:amount/confirm", func(c *gin.Context) {
		from := c.Param("from")
		to := c.Param("to")
		amountStr := c.Param("amount")

		amount, err := strconv.Atoi(amountStr)
		if err != nil || amount <= 0 {
			c.HTML(400, "transfer.html", gin.H{
				"FromUser": from,
				"ToUser":   to,
				"Amount":   amountStr,
				"Error":    "Invalid amount",
			})
			return
		}

		if !cfg.TestMode {
			c.HTML(200, "transfer.html", gin.H{
				"FromUser": from,
				"ToUser":   to,
				"Amount":   amountStr,
				"Error":    "OIDC authentication not yet implemented",
			})
			return
		}

		err = transferService.Transfer(from, to, amount, true)
		if err != nil {
			c.HTML(400, "transfer.html", gin.H{
				"FromUser": from,
				"ToUser":   to,
				"Amount":   amountStr,
				"Error":    err.Error(),
			})
			return
		}

		c.HTML(200, "transfer.html", gin.H{
			"FromUser": from,
			"ToUser":   to,
			"Amount":   amountStr,
			"Success":  true,
		})
	})

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	api := router.Group("/api/v1")
	{
		api.GET("/total", publicHandler.GetTotalBeans)

		authenticated := api.Group("")
		if cfg.TestMode {
			authenticated.Use(authMiddleware.RequireAuth())
		} else {
			authenticated.Use(logtoHandler.RequireAuth())
		}
		{
			authenticated.GET("/wallet", walletHandler.GetWallet)
			authenticated.GET("/transactions", walletHandler.GetTransactions)
			authenticated.POST("/transfer", transferHandler.Transfer)

			authenticated.POST("/tokens", tokenHandler.CreateToken)
			authenticated.GET("/tokens", tokenHandler.ListTokens)
			authenticated.DELETE("/tokens/:id", tokenHandler.DeleteToken)
		}

		admin := api.Group("/admin")
		if cfg.TestMode {
			admin.Use(authMiddleware.RequireAuth())
		} else {
			admin.Use(logtoHandler.RequireAuth())
		}
		admin.Use(adminMiddleware.RequireAdmin())
		{
			admin.GET("/users", adminHandler.ListUsers)
			admin.GET("/transactions", adminHandler.ListAllTransactions)
			admin.PUT("/wallet/:username", adminHandler.UpdateWallet)
		}
	}

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Starting Beapin server on %s", addr)
	if cfg.TestMode {
		log.Println("⚠️  TEST MODE ENABLED - Authentication bypassed")
	}
	log.Fatal(router.Run(addr))
}
