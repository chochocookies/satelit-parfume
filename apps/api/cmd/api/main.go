// Command api boots the Satelit Parfume backend: it wires up config,
// connects to PostgreSQL and Redis, constructs every module's
// repository/service/handler chain, mounts the router, and serves HTTP
// until it receives a shutdown signal.
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"satelit-parfume-api/internal/auth"
	"satelit-parfume-api/internal/branches"
	"satelit-parfume-api/internal/cart"
	"satelit-parfume-api/internal/categories"
	"satelit-parfume-api/internal/customers"
	"satelit-parfume-api/internal/dashboard"
	"satelit-parfume-api/internal/health"
	"satelit-parfume-api/internal/inventory"
	"satelit-parfume-api/internal/orders"
	"satelit-parfume-api/internal/payments"
	"satelit-parfume-api/internal/payments/duitku"
	"satelit-parfume-api/internal/products"
	"satelit-parfume-api/internal/shifts"
	"satelit-parfume-api/internal/stock"
	"satelit-parfume-api/internal/users"
	"satelit-parfume-api/pkg/database"
	"satelit-parfume-api/pkg/jwt"
	"satelit-parfume-api/pkg/logger"
	"satelit-parfume-api/pkg/response"
)

type config struct {
	Port        string
	GinMode     string
	DatabaseURL string
	RedisURL    string
	CORSOrigins string

	JWTSecret        string
	JWTRefreshSecret string
	AccessTokenTTL   time.Duration
	RefreshTokenTTL  time.Duration

	// PublicAPIURL/PublicWebURL let the backend construct URLs *about
	// itself* (the webhook callback) and about the frontend (the
	// post-payment return page) — a container has no way to know its
	// own externally-reachable address on its own. Both default to
	// localhost for local dev; in any real deployment these need to be
	// real public HTTPS URLs, or Duitku's servers can't reach the
	// callback at all (see the root README's payment section for how to
	// test this locally with a tunnel).
	PublicAPIURL string
	PublicWebURL string

	DuitkuMerchantCode string
	DuitkuMerchantKey  string
	DuitkuBaseURL      string
}

func loadConfig() config {
	return config{
		Port:        getEnv("PORT", "8080"),
		GinMode:     getEnv("GIN_MODE", gin.ReleaseMode),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://satelit:satelit@localhost:5432/satelit_parfume?sslmode=disable"),
		RedisURL:    getEnv("REDIS_URL", "redis://localhost:6379/0"),
		CORSOrigins: getEnv("CORS_ORIGINS", "http://localhost:3000"),

		JWTSecret:        getEnv("JWT_SECRET", "change-me-in-production"),
		JWTRefreshSecret: getEnv("JWT_REFRESH_SECRET", "change-me-in-production"),
		AccessTokenTTL:   getEnvMinutes("ACCESS_TOKEN_TTL_MINUTES", 15),
		RefreshTokenTTL:  getEnvHours("REFRESH_TOKEN_TTL_HOURS", 24*7),

		PublicAPIURL: getEnv("PUBLIC_API_URL", "http://localhost:8080"),
		PublicWebURL: getEnv("PUBLIC_WEB_URL", "http://localhost:3000"),

		DuitkuMerchantCode: getEnv("DUITKU_MERCHANT_CODE", ""),
		DuitkuMerchantKey:  getEnv("DUITKU_MERCHANT_KEY", ""),
		DuitkuBaseURL:      getEnv("DUITKU_BASE_URL", duitku.DefaultSandboxBaseURL),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvMinutes(key string, fallbackMinutes int) time.Duration {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return time.Duration(n) * time.Minute
		}
	}
	return time.Duration(fallbackMinutes) * time.Minute
}

func getEnvHours(key string, fallbackHours int) time.Duration {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return time.Duration(n) * time.Hour
		}
	}
	return time.Duration(fallbackHours) * time.Hour
}

func main() {
	// .env is only present in local, non-Docker development; Docker Compose
	// and production inject real environment variables, so a missing file
	// here is expected and not fatal.
	_ = godotenv.Load()

	logger.Init()
	cfg := loadConfig()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pg, err := database.NewPostgresPool(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Errorf("postgres connection failed: %v", err)
		os.Exit(1)
	}
	defer pg.Close()
	logger.Infof("connected to postgres")

	rdb, err := database.NewRedisClient(cfg.RedisURL)
	if err != nil {
		logger.Errorf("redis connection failed: %v", err)
		os.Exit(1)
	}
	defer rdb.Close()
	logger.Infof("connected to redis")

	if cfg.JWTSecret == "change-me-in-production" || cfg.JWTRefreshSecret == "change-me-in-production" {
		logger.Infof("WARNING: using default JWT secrets — fine for local dev, never for production")
	}

	// ── Wiring: repository → service → handler, per module ──
	accessTokens := jwt.NewManager(cfg.JWTSecret, cfg.AccessTokenTTL)
	refreshTokens := jwt.NewManager(cfg.JWTRefreshSecret, cfg.RefreshTokenTTL)

	usersRepo := users.NewRepository(pg)
	usersHandler := users.NewHandler(usersRepo)
	customersRepo := customers.NewRepository(pg)
	authRepo := auth.NewRepository(pg)

	authService := auth.NewService(auth.ServiceDeps{
		Users:               usersRepo,
		Customers:           customersRepo,
		RefreshTokens:       authRepo,
		AccessTokenManager:  accessTokens,
		RefreshTokenManager: refreshTokens,
		Redis:               rdb,
	})
	authHandler := auth.NewHandler(authService)

	categoriesRepo := categories.NewRepository(pg)
	categoriesHandler := categories.NewHandler(categoriesRepo)

	inventoryRepo := inventory.NewRepository(pg)
	inventoryHandler := inventory.NewHandler(inventoryRepo)

	branchesRepo := branches.NewRepository(pg)
	branchesHandler := branches.NewHandler(branchesRepo)

	productsRepo := products.NewRepository(pg)
	productsHandler := products.NewHandler(productsRepo, categoriesRepo, inventoryRepo)

	cartRepo := cart.NewRepository(pg)
	cartService := cart.NewService(cartRepo, inventoryRepo)
	cartHandler := cart.NewHandler(cartService)

	ordersRepo := orders.NewRepository(pg)
	shiftsRepo := shifts.NewRepository(pg)
	shiftsHandler := shifts.NewHandler(shiftsRepo)
	stockRepo := stock.NewRepository(pg)
	stockHandler := stock.NewHandler(stockRepo)
	ordersService := orders.NewService(ordersRepo, cartService, inventoryRepo, shiftsRepo, stockRepo)
	ordersHandler := orders.NewHandler(ordersService)

	// Duitku is wired up even with empty credentials — CreatePayment will
	// simply fail with a clear "no reference in response" or a request
	// error against the sandbox until DUITKU_MERCHANT_CODE/KEY are set,
	// rather than the whole API refusing to boot over an optional
	// integration nobody's configured yet.
	duitkuAdapter := duitku.NewAdapter(cfg.DuitkuMerchantCode, cfg.DuitkuMerchantKey, cfg.DuitkuBaseURL)
	paymentsRepo := payments.NewRepository(pg)
	paymentsService := payments.NewService(paymentsRepo, duitkuAdapter, ordersService)
	paymentsCallbackURL := cfg.PublicAPIURL + "/api/v1/payments/webhook/duitku"
	paymentsReturnURL := cfg.PublicWebURL + "/shop"
	paymentsHandler := payments.NewHandler(paymentsService, ordersService, paymentsCallbackURL, paymentsReturnURL)

	// dashboard reads across products/orders/branches/users/inventory's
	// tables directly rather than through each of those repositories —
	// see the package doc comment on why "count everything" doesn't
	// belong to any one of them.
	dashboardRepo := dashboard.NewRepository(pg)
	dashboardHandler := dashboard.NewHandler(dashboardRepo)

	gin.SetMode(cfg.GinMode)
	router := gin.New()
	router.Use(gin.Recovery(), logger.GinMiddleware(), corsMiddleware(cfg.CORSOrigins))

	// Root-level health check (container/orchestrator probes).
	router.GET("/health", health.Handler(pg, rdb))

	// Versioned API surface. Every module mounts its routes under this group.
	v1 := router.Group("/api/v1")
	{
		v1.GET("/health", health.Handler(pg, rdb))

		authGroup := v1.Group("/auth")
		{
			authGroup.POST("/register", authHandler.Register)
			authGroup.POST("/login", authHandler.Login)
			authGroup.POST("/staff/login", authHandler.StaffLogin)
			authGroup.POST("/refresh", authHandler.Refresh)
			authGroup.POST("/logout", authHandler.Logout)
			authGroup.GET("/me", auth.RequireAuth(accessTokens), authHandler.Me)
		}

		// Public catalog reads — no auth required, matches section 15/16/17.
		v1.GET("/products", productsHandler.List)
		v1.GET("/products/:slug", productsHandler.GetBySlug)
		v1.GET("/categories", categoriesHandler.List)

		// Public branch reads — the future branch selector (section 10)
		// and nearest-store lookup (?lat=&lng=, section 11) hang off these.
		v1.GET("/branches", branchesHandler.List)
		v1.GET("/branches/:slug", branchesHandler.GetBySlug)

		// Cart — works for both guests (X-Cart-Token header) and logged-in
		// customers (Bearer token); OptionalAuth never rejects a request
		// for lacking one, unlike RequireAuth elsewhere.
		cartGroup := v1.Group("/cart")
		cartGroup.Use(auth.OptionalAuth(accessTokens))
		{
			cartGroup.GET("", cartHandler.Get)
			cartGroup.POST("/items", cartHandler.AddItem)
			cartGroup.PUT("/items/:itemId", cartHandler.UpdateItem)
			cartGroup.DELETE("/items/:itemId", cartHandler.RemoveItem)
			cartGroup.DELETE("", cartHandler.Clear)
		}

		// Orders — checkout uses the same optional-auth pattern as cart
		// (guest or customer, no login required either way); everything
		// past that point needs a real customer identity, since "my
		// orders" is meaningless for a guest with no durable session —
		// guests track their order via Lookup (order number + phone)
		// instead.
		v1.POST("/orders", auth.OptionalAuth(accessTokens), ordersHandler.Checkout)
		v1.POST("/orders/lookup", ordersHandler.Lookup)
		ordersGroup := v1.Group("/orders")
		ordersGroup.Use(auth.RequireAuth(accessTokens))
		{
			ordersGroup.GET("", ordersHandler.ListMine)
			ordersGroup.GET("/:id", ordersHandler.GetMine)
			ordersGroup.POST("/:id/cancel", ordersHandler.CancelMine)
			ordersGroup.POST("/:id/pay", paymentsHandler.Pay)
			ordersGroup.GET("/:id/payment", paymentsHandler.GetForOrder)
		}

		// Payment webhook — deliberately outside every auth group. See
		// Handler.Webhook's own doc comment for why: a gateway calling
		// back has no Bearer token to present, and trust here comes from
		// signature verification (section 26), not network-level auth.
		v1.POST("/payments/webhook/duitku", paymentsHandler.Webhook)

		// Admin-only catalog + branch management (SUPER_ADMIN/ADMIN only —
		// creating a branch or reassigning staff is an org-structure
		// decision, not something a single branch's own staff should do).
		adminGroup := v1.Group("/admin")
		adminGroup.Use(auth.RequireAuth(accessTokens), auth.RequireRole("SUPER_ADMIN", "ADMIN"))
		{
			adminGroup.POST("/products/import", productsHandler.Import)
			adminGroup.GET("/products", productsHandler.AdminList)
			adminGroup.POST("/products", productsHandler.AdminCreate)
			adminGroup.PUT("/products/:id", productsHandler.AdminUpdate)
			adminGroup.DELETE("/products/:id", productsHandler.AdminDelete)

			adminGroup.GET("/ping", func(c *gin.Context) {
				response.OK(c, http.StatusOK, "pong", nil)
			})

			adminGroup.POST("/branches", branchesHandler.Create)
			adminGroup.PUT("/branches/:id", branchesHandler.Update)
			adminGroup.DELETE("/branches/:id", branchesHandler.Delete)
			adminGroup.POST("/branches/:id/staff", branchesHandler.AssignStaff)
			adminGroup.DELETE("/branches/:id/staff/:userId", branchesHandler.UnassignStaff)

			// Cross-branch order view — see orders.Handler.AdminList's doc
			// comment for how this differs from the branch-scoped
			// /admin/branches/:id/orders routes below.
			adminGroup.GET("/orders", ordersHandler.AdminList)
			adminGroup.GET("/orders/:id", ordersHandler.AdminGet)
			adminGroup.PUT("/orders/:id/status", ordersHandler.AdminUpdateStatus)

			// Staff account management — the "create staff user" endpoint
			// internal/users' repository doc comment (pre-Phase-9) flagged
			// as missing. See users.Handler.Create for the SUPER_ADMIN
			// safeguard on granting the SUPER_ADMIN role itself.
			adminGroup.GET("/users", usersHandler.List)
			adminGroup.POST("/users", usersHandler.Create)
			adminGroup.GET("/users/:id", usersHandler.Get)
			adminGroup.PUT("/users/:id", usersHandler.Update)

			// Dashboard overview stats — see internal/dashboard's package
			// doc comment.
			adminGroup.GET("/dashboard/stats", dashboardHandler.Stats)

			// Phase 11: a transfer spans two branches, so its detail
			// doesn't belong under either one's URL alone. Branch-level
			// staff already see full transfer detail (items included)
			// through the branch-scoped list above; this is purely a
			// SUPER_ADMIN/ADMIN convenience for looking one up by id
			// directly.
			adminGroup.GET("/transfers/:transferId", stockHandler.GetTransfer)

			// Maintenance: releases stock held by PENDING_PAYMENT orders
			// whose reservation window elapsed with nobody looking at them
			// (GetByID already expires one lazily on read — this catches
			// the rest). Meant to be hit periodically by a cron/scheduler;
			// see the root README.
			adminGroup.POST("/orders/sweep-expired", ordersHandler.SweepExpired)
		}

		// Branch-scoped routes: role sets differ per route (read is wider
		// than write, and both differ from adminGroup's fixed
		// SUPER_ADMIN/ADMIN), so these are registered directly rather than
		// through adminGroup. branches.RequireBranchAccess is what actually
		// enforces "your role qualifies, but only for a branch you're
		// assigned to" (section 9/37).
		v1.GET("/admin/branches/:id/staff",
			auth.RequireAuth(accessTokens),
			auth.RequireRole("SUPER_ADMIN", "ADMIN", "BRANCH_MANAGER"),
			branches.RequireBranchAccess(branchesRepo),
			branchesHandler.ListStaff,
		)
		v1.GET("/admin/branches/:id/inventory",
			auth.RequireAuth(accessTokens),
			auth.RequireRole("SUPER_ADMIN", "ADMIN", "BRANCH_MANAGER", "CASHIER", "INVENTORY_STAFF"),
			branches.RequireBranchAccess(branchesRepo),
			inventoryHandler.ListForBranch,
		)
		v1.PUT("/admin/branches/:id/inventory/:variantId",
			auth.RequireAuth(accessTokens),
			auth.RequireRole("SUPER_ADMIN", "ADMIN", "BRANCH_MANAGER", "INVENTORY_STAFF"), // not CASHIER — selling isn't adjusting stock
			branches.RequireBranchAccess(branchesRepo),
			inventoryHandler.SetStock,
		)
		v1.GET("/admin/branches/:id/orders",
			auth.RequireAuth(accessTokens),
			auth.RequireRole("SUPER_ADMIN", "ADMIN", "BRANCH_MANAGER", "CASHIER"),
			branches.RequireBranchAccess(branchesRepo),
			ordersHandler.ListForBranch,
		)
		v1.GET("/admin/branches/:id/orders/:orderId",
			auth.RequireAuth(accessTokens),
			auth.RequireRole("SUPER_ADMIN", "ADMIN", "BRANCH_MANAGER", "CASHIER"),
			branches.RequireBranchAccess(branchesRepo),
			ordersHandler.GetForBranch,
		)
		v1.PUT("/admin/branches/:id/orders/:orderId/status",
			auth.RequireAuth(accessTokens),
			auth.RequireRole("SUPER_ADMIN", "ADMIN", "BRANCH_MANAGER", "CASHIER"), // cashier confirms cash payment at POS — section 28
			branches.RequireBranchAccess(branchesRepo),
			ordersHandler.UpdateStatus,
		)

		// Phase 10: POS — cashier till sessions and the sale itself. Same
		// wide, operational role set as the orders/inventory routes just
		// above (a cashier's day-to-day job, not an org-structure change),
		// registered directly on v1 for the same reason those are.
		v1.POST("/admin/branches/:id/shifts",
			auth.RequireAuth(accessTokens),
			auth.RequireRole("SUPER_ADMIN", "ADMIN", "BRANCH_MANAGER", "CASHIER"),
			branches.RequireBranchAccess(branchesRepo),
			shiftsHandler.Open,
		)
		v1.GET("/admin/branches/:id/shifts/current",
			auth.RequireAuth(accessTokens),
			auth.RequireRole("SUPER_ADMIN", "ADMIN", "BRANCH_MANAGER", "CASHIER"),
			branches.RequireBranchAccess(branchesRepo),
			shiftsHandler.Current,
		)
		v1.GET("/admin/branches/:id/shifts",
			auth.RequireAuth(accessTokens),
			auth.RequireRole("SUPER_ADMIN", "ADMIN", "BRANCH_MANAGER"), // reconciliation history — not a cashier's own mid-shift concern
			branches.RequireBranchAccess(branchesRepo),
			shiftsHandler.List,
		)
		v1.PUT("/admin/branches/:id/shifts/:shiftId/close",
			auth.RequireAuth(accessTokens),
			auth.RequireRole("SUPER_ADMIN", "ADMIN", "BRANCH_MANAGER", "CASHIER"),
			branches.RequireBranchAccess(branchesRepo),
			shiftsHandler.Close,
		)
		v1.POST("/admin/branches/:id/pos/checkout",
			auth.RequireAuth(accessTokens),
			auth.RequireRole("SUPER_ADMIN", "ADMIN", "BRANCH_MANAGER", "CASHIER"),
			branches.RequireBranchAccess(branchesRepo),
			ordersHandler.POSCheckout,
		)
		v1.POST("/admin/branches/:id/orders/:orderId/pay",
			auth.RequireAuth(accessTokens),
			auth.RequireRole("SUPER_ADMIN", "ADMIN", "BRANCH_MANAGER", "CASHIER"),
			branches.RequireBranchAccess(branchesRepo),
			paymentsHandler.AdminPay,
		)

		// Phase 11: advanced inventory — movement ledger, transfers,
		// opname. Same role set as inventory.Handler.SetStock (not
		// CASHIER — this is stock management, not sales).
		v1.POST("/admin/branches/:id/inventory/:variantId/receive",
			auth.RequireAuth(accessTokens),
			auth.RequireRole("SUPER_ADMIN", "ADMIN", "BRANCH_MANAGER", "INVENTORY_STAFF"),
			branches.RequireBranchAccess(branchesRepo),
			stockHandler.Receive,
		)
		v1.POST("/admin/branches/:id/inventory/:variantId/adjust",
			auth.RequireAuth(accessTokens),
			auth.RequireRole("SUPER_ADMIN", "ADMIN", "BRANCH_MANAGER", "INVENTORY_STAFF"),
			branches.RequireBranchAccess(branchesRepo),
			stockHandler.Adjust,
		)
		v1.GET("/admin/branches/:id/inventory/:variantId/movements",
			auth.RequireAuth(accessTokens),
			auth.RequireRole("SUPER_ADMIN", "ADMIN", "BRANCH_MANAGER", "INVENTORY_STAFF"),
			branches.RequireBranchAccess(branchesRepo),
			stockHandler.Movements,
		)

		v1.POST("/admin/branches/:id/transfers",
			auth.RequireAuth(accessTokens),
			auth.RequireRole("SUPER_ADMIN", "ADMIN", "BRANCH_MANAGER", "INVENTORY_STAFF"),
			branches.RequireBranchAccess(branchesRepo),
			stockHandler.CreateTransfer,
		)
		v1.GET("/admin/branches/:id/transfers",
			auth.RequireAuth(accessTokens),
			auth.RequireRole("SUPER_ADMIN", "ADMIN", "BRANCH_MANAGER", "INVENTORY_STAFF"),
			branches.RequireBranchAccess(branchesRepo),
			stockHandler.ListTransfers,
		)
		// Only the destination branch can complete (they're physically
		// receiving the goods); only the source can cancel (they
		// requested it) — CompleteTransfer/CancelTransfer each check
		// :id against the transfer's actual to/from branch themselves.
		v1.PUT("/admin/branches/:id/transfers/:transferId/complete",
			auth.RequireAuth(accessTokens),
			auth.RequireRole("SUPER_ADMIN", "ADMIN", "BRANCH_MANAGER", "INVENTORY_STAFF"),
			branches.RequireBranchAccess(branchesRepo),
			stockHandler.CompleteTransfer,
		)
		v1.PUT("/admin/branches/:id/transfers/:transferId/cancel",
			auth.RequireAuth(accessTokens),
			auth.RequireRole("SUPER_ADMIN", "ADMIN", "BRANCH_MANAGER", "INVENTORY_STAFF"),
			branches.RequireBranchAccess(branchesRepo),
			stockHandler.CancelTransfer,
		)

		v1.POST("/admin/branches/:id/opnames",
			auth.RequireAuth(accessTokens),
			auth.RequireRole("SUPER_ADMIN", "ADMIN", "BRANCH_MANAGER", "INVENTORY_STAFF"),
			branches.RequireBranchAccess(branchesRepo),
			stockHandler.StartOpname,
		)
		v1.GET("/admin/branches/:id/opnames/current",
			auth.RequireAuth(accessTokens),
			auth.RequireRole("SUPER_ADMIN", "ADMIN", "BRANCH_MANAGER", "INVENTORY_STAFF"),
			branches.RequireBranchAccess(branchesRepo),
			stockHandler.CurrentOpname,
		)
		v1.GET("/admin/branches/:id/opnames",
			auth.RequireAuth(accessTokens),
			auth.RequireRole("SUPER_ADMIN", "ADMIN", "BRANCH_MANAGER", "INVENTORY_STAFF"),
			branches.RequireBranchAccess(branchesRepo),
			stockHandler.ListOpnames,
		)
		v1.GET("/admin/branches/:id/opnames/:opnameId",
			auth.RequireAuth(accessTokens),
			auth.RequireRole("SUPER_ADMIN", "ADMIN", "BRANCH_MANAGER", "INVENTORY_STAFF"),
			branches.RequireBranchAccess(branchesRepo),
			stockHandler.GetOpname,
		)
		v1.PUT("/admin/branches/:id/opnames/:opnameId/items/:itemId",
			auth.RequireAuth(accessTokens),
			auth.RequireRole("SUPER_ADMIN", "ADMIN", "BRANCH_MANAGER", "INVENTORY_STAFF"),
			branches.RequireBranchAccess(branchesRepo),
			stockHandler.CountItem,
		)
		v1.PUT("/admin/branches/:id/opnames/:opnameId/complete",
			auth.RequireAuth(accessTokens),
			auth.RequireRole("SUPER_ADMIN", "ADMIN", "BRANCH_MANAGER", "INVENTORY_STAFF"),
			branches.RequireBranchAccess(branchesRepo),
			stockHandler.CompleteOpname,
		)
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Infof("satelit parfume api listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("server error: %v", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Infof("shutdown signal received, draining connections...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Errorf("forced shutdown: %v", err)
	}
}

// corsMiddleware is a minimal, explicit CORS layer. Revisit once the
// frontend actually sends credentialed requests (Phase 5) — it may need
// Access-Control-Allow-Credentials and a stricter origin check than a
// single configured string.
//
// Bug fix (found while helping debug a real local run of this repo,
// after Phase 11): Access-Control-Allow-Headers never included
// X-Cart-Token, the custom header cartHeaders() (lib/api-client.ts, since
// Phase 6) attaches to every cart/checkout/POS-checkout request. Any
// browser sending a non-simple header needs it explicitly allowed here or
// the preflight fails and the browser blocks the real request — which
// surfaces to the user as a bare "Failed to fetch", no HTTP status to
// even inspect. This went uncaught through Phases 6-11 because nothing
// in any of those phases' verification actually ran a live browser
// against a live backend at the same time; this is the first real
// end-to-end run that could have caught it.
func corsMiddleware(origin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Cart-Token")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
