package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/h4ks-com/bean-bank/internal/auth"
	"github.com/h4ks-com/bean-bank/internal/config"
	"github.com/h4ks-com/bean-bank/internal/database"
	"github.com/h4ks-com/bean-bank/internal/handlers"
	"github.com/h4ks-com/bean-bank/internal/middleware"
	"github.com/h4ks-com/bean-bank/internal/repository"
	"github.com/h4ks-com/bean-bank/internal/services"

	_ "github.com/h4ks-com/bean-bank/docs"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title           Bean Bank API
// @version         1.0
// @description     Bean currency management system for h4ks.com
// @BasePath        /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Bearer token authentication. Format: "Bearer {token}"
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
	harvestRepo := repository.NewHarvestRepository(db)

	walletService := services.NewWalletService(userRepo, transactionRepo)
	transferService := services.NewTransferService(userRepo, transactionRepo, db)
	tokenService := services.NewTokenService(tokenRepo, userRepo, cfg.JWT.Secret)
	harvestService := services.NewHarvestService(harvestRepo, userRepo, transactionRepo, db)
	exportService := services.NewExportService(userRepo, transactionRepo, cfg.ExportSigningKey)

	authMiddleware := middleware.NewAuthMiddleware(tokenService, cfg.TestMode)
	adminMiddleware := middleware.NewAdminMiddleware(cfg.AdminUsers)
	logtoHandler := auth.NewLogtoHandler(&cfg.Logto)

	walletHandler := handlers.NewWalletHandler(walletService)
	transferHandler := handlers.NewTransferHandler(transferService)
	tokenHandler := handlers.NewTokenHandler(tokenService)
	adminHandler := handlers.NewAdminHandler(userRepo, transactionRepo, walletService)
	publicHandler := handlers.NewPublicHandler(walletService, harvestService)
	harvestHandler := handlers.NewHarvestHandler(harvestService)
	exportHandler := handlers.NewExportHandler(exportService)
	browserHandler := handlers.NewBrowserHandler(walletService, transferService, tokenService, logtoHandler)

	router := gin.Default()

	store := cookie.NewStore([]byte(cfg.Session.Secret))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 7 days
		HttpOnly: true,
		Secure:   cfg.Session.Secure,
		SameSite: http.SameSiteLaxMode,
	})
	router.Use(sessions.Sessions("beapin_session", store))

	router.LoadHTMLGlob("web/templates/*")
	router.Static("/static", "./web/static")

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
					if claims.Username != "" {
						username = claims.Username
					}
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

	router.GET("/wallet", func(c *gin.Context) {
		isAuthenticated := false
		username := ""
		if !cfg.TestMode {
			logtoClient := logtoHandler.CreateLogtoClient(c)
			isAuthenticated = logtoClient.IsAuthenticated()
			if !isAuthenticated {
				c.Redirect(http.StatusFound, "/auth/login")
				return
			}
			if claims, err := logtoClient.GetIdTokenClaims(); err == nil {
				username = claims.Sub
				if claims.Username != "" {
					username = claims.Username
				}
			}
		} else {
			isAuthenticated = true
			username = "test_user"
		}

		isAdmin := false
		for _, admin := range cfg.AdminUsers {
			if admin == username {
				isAdmin = true
				break
			}
		}

		c.HTML(200, "wallet.html", gin.H{
			"IsAuthenticated": isAuthenticated,
			"Username":        username,
			"TestMode":        cfg.TestMode,
			"IsAdmin":         isAdmin,
		})
	})

	router.GET("/transfer/:from/:to/:amount", func(c *gin.Context) {
		from := c.Param("from")
		to := c.Param("to")
		amount := c.Param("amount")

		isAuthenticated := cfg.TestMode
		var currentUser string

		if !cfg.TestMode {
			var ok bool
			currentUser, ok = logtoHandler.GetAuthenticatedUser(c)
			isAuthenticated = ok
		} else {
			currentUser = from
		}

		data := gin.H{
			"FromUser": from,
			"ToUser":   to,
			"Amount":   amount,
		}

		if !isAuthenticated {
			data["NeedsAuth"] = true
		} else if currentUser != from {
			data["Error"] = "You can only send beans from your own account"
		}

		c.HTML(200, "transfer.html", data)
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

		var currentUser string
		if !cfg.TestMode {
			var ok bool
			currentUser, ok = logtoHandler.GetAuthenticatedUser(c)
			if !ok {
				c.HTML(401, "transfer.html", gin.H{
					"FromUser":  from,
					"ToUser":    to,
					"Amount":    amountStr,
					"NeedsAuth": true,
				})
				return
			}
		} else {
			currentUser = from
		}

		if currentUser != from {
			c.HTML(403, "transfer.html", gin.H{
				"FromUser": from,
				"ToUser":   to,
				"Amount":   amountStr,
				"Error":    "You can only send beans from your own account",
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

	router.GET("/swagger/*any", func(c *gin.Context) {
		path := c.Param("any")
		if path == "/" || path == "/index.html" {
			handlers.SwaggerUIWithBearerFix()(c)
		} else {
			ginSwagger.WrapHandler(swaggerFiles.Handler)(c)
		}
	})

	browser := router.Group("/browser")
	if !cfg.TestMode {
		browser.Use(logtoHandler.RequireAuth())
	}
	{
		browser.GET("/wallet", browserHandler.GetWallet)
		browser.GET("/transactions", browserHandler.GetTransactions)
		browser.GET("/transactions/export", exportHandler.ExportTransactions)
		browser.POST("/transfer", browserHandler.Transfer)
		browser.POST("/tokens", browserHandler.CreateToken)
		browser.GET("/tokens", browserHandler.ListTokens)
		browser.DELETE("/tokens/:id", browserHandler.DeleteToken)
		browser.GET("/users/search", adminHandler.SearchUsers)

		browserAdmin := browser.Group("/admin")
		if !cfg.TestMode {
			browserAdmin.Use(adminMiddleware.RequireAdmin())
		}
		{
			browserAdmin.GET("/harvests", harvestHandler.GetAllHarvests)
			browserAdmin.POST("/harvests", harvestHandler.CreateHarvest)
			browserAdmin.PUT("/harvests/:id", harvestHandler.UpdateHarvest)
			browserAdmin.DELETE("/harvests/:id", harvestHandler.DeleteHarvest)
			browserAdmin.POST("/harvests/:id/assign", harvestHandler.AssignUser)
			browserAdmin.POST("/harvests/:id/complete", harvestHandler.CompleteHarvest)
		}
	}

	api := router.Group("/api/v1")
	{
		api.GET("/total", publicHandler.GetTotalBeans)
		api.GET("/leaderboard", publicHandler.GetLeaderboard)
		api.GET("/harvests", publicHandler.GetHarvests)
		api.POST("/transactions/verify", exportHandler.VerifyExport)

		authenticated := api.Group("")
		authenticated.Use(authMiddleware.RequireAuth())
		{
			authenticated.GET("/wallet", walletHandler.GetWallet)
			authenticated.GET("/transactions", walletHandler.GetTransactions)
			authenticated.POST("/transfer", transferHandler.Transfer)
			authenticated.GET("/transactions/export", exportHandler.ExportTransactions)

			authenticated.POST("/tokens", tokenHandler.CreateToken)
			authenticated.GET("/tokens", tokenHandler.ListTokens)
			authenticated.DELETE("/tokens/:id", tokenHandler.DeleteToken)
			authenticated.GET("/users/search", adminHandler.SearchUsers)
		}

		admin := api.Group("/admin")
		admin.Use(authMiddleware.RequireAuth())
		admin.Use(adminMiddleware.RequireAdmin())
		{
			admin.GET("/users", adminHandler.ListUsers)
			admin.GET("/transactions", adminHandler.ListAllTransactions)
			admin.PUT("/wallet/:username", adminHandler.UpdateWallet)

			admin.GET("/harvests", harvestHandler.GetAllHarvests)
			admin.POST("/harvests", harvestHandler.CreateHarvest)
			admin.PUT("/harvests/:id", harvestHandler.UpdateHarvest)
			admin.DELETE("/harvests/:id", harvestHandler.DeleteHarvest)
			admin.POST("/harvests/:id/assign", harvestHandler.AssignUser)
			admin.POST("/harvests/:id/complete", harvestHandler.CompleteHarvest)
		}
	}

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Starting Beapin server on %s", addr)
	if cfg.TestMode {
		log.Println("⚠️  TEST MODE ENABLED - Authentication bypassed")
	}
	log.Fatal(router.Run(addr))
}
