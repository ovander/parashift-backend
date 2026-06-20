package main

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migratepostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	sentry "github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
	glogger "gorm.io/gorm/logger"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/ovander/backendkit/gormlogger"
	"github.com/ovander/backendkit/httpware"
	"github.com/ovander/backendkit/jwtauth"

	"github.com/ovander/parashift/internal/config"
	"github.com/ovander/parashift/internal/handler"
	"github.com/ovander/parashift/internal/middleware"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/repo"
	"github.com/ovander/parashift/internal/router"
	"github.com/ovander/parashift/internal/service"
)

// AppResources holds all live application resources for clean shutdown.
type AppResources struct {
	DB       *gorm.DB
	Services *service.ServiceBundle
	Server   *http.Server
	Limiter  *httpware.RateLimiter
}

// initSentry configures the Sentry SDK when SENTRY_DSN is set.
// A missing DSN is not an error — monitoring is simply disabled.
func initSentry(cfg *config.Config, logger *logrus.Logger) {
	if cfg.Sentry.DSN == "" {
		logger.Info("SENTRY_DSN not set — error monitoring disabled")
		return
	}
	if err := sentry.Init(sentry.ClientOptions{
		Dsn:              cfg.Sentry.DSN,
		Environment:      cfg.Env,
		AttachStacktrace: true,
		TracesSampleRate: 0.1,
	}); err != nil {
		// Non-fatal: log and continue — the app runs fine without Sentry.
		logger.WithError(err).Warn("Sentry initialisation failed")
		return
	}
	logger.WithField("env", cfg.Env).Info("Sentry error monitoring active")
}

// bootstrap initialises all application layers in strict dependency order.
func bootstrap(cfg *config.Config, logger *logrus.Logger, build handler.BuildInfo) (*AppResources, error) {
	entry := logger.WithField("component", "bootstrap")

	// Step 2.5: Sentry — init before any application code so bootstrap panics are captured
	initSentry(cfg, logger)

	// Step 3: Socrate connectivity checks
	pingSocrate(cfg, entry)
	pingSocrateAdmin(cfg, entry)

	// Step 4: Database connection.
	// (Auth middleware is built later, after services, so its revocation check can
	// use the revocation service.)
	entry.Info("connecting to database")
	db, err := initDB(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("db: %w", err)
	}

	// Step 5: File-based SQL migrations (golang-migrate).
	// Controlled by RUN_MIGRATIONS=true — set this in your deploy pipeline (e.g.
	// as a one-shot init container or pre-deploy job) rather than having every
	// replica run migrations on startup. Completely independent of AutoMigrate.
	if cfg.RunMigrations {
		if err := runMigrations(cfg, db); err != nil {
			return nil, fmt.Errorf("migrations: %w", err)
		}
	} else {
		entry.Info("RUN_MIGRATIONS=false — skipping file-based SQL migrations")
	}

	// Step 6: GORM AutoMigrate (dev/staging only — never in production).
	if cfg.AutoMigrate {
		entry.Info("running AutoMigrate")
		if err := autoMigrate(db); err != nil {
			logger.WithError(err).Warn("AutoMigrate completed with warnings")
		}
	}

	// Steps 7-9
	repos    := repo.NewRepoBundle(db)
	services := service.NewServiceBundle(repos, logger.WithField("component", "service"), cfg)
	handlers := handler.NewHandlerBundle(services, cfg, db, build)

	// Step 9b: JWT auth (JWKS) — built after services so it can enforce revocation.
	//   - WithAudience rejects tokens not minted for this app's client_id, so a token
	//     issued for another service on the same Socrate issuer cannot be replayed here.
	//   - WithRevocationCheck rejects tokens issued before a subject's revocation floor,
	//     so logout / password-change / admin-revoke take effect before token expiry.
	// (backendkit v1.8.0)
	jwtMW := jwtauth.New(cfg.JWKS.URL, cfg.JWKS.Issuer, entry,
		jwtauth.WithAudience(cfg.Socrate.ClientID),
		jwtauth.WithRevocationCheck(services.Revocation.CheckToken))

	// Step 10: Middleware
	tenantMW := middleware.NewTenantMiddleware(repos.Employee, logger.WithField("component", "tenant"), cfg.Socrate.BaseURL)
	rbacMW   := middleware.NewRBACMiddleware(logger.WithField("component", "rbac"))
	limiter  := httpware.NewRateLimiter(100, 200)

	mw := router.Middleware{
		Auth:           jwtMW.Handler,
		Tenant:         tenantMW,
		RBAC:           rbacMW,
		GeneralLimiter: limiter,
		Logger:         logger,  // *logrus.Logger for httpware.Logger
	}

	// Step 11-12: Router + Server
	r := router.NewRouter(cfg, handlers, mw)

	// Wrap outermost handler with Sentry HTTP middleware when DSN is configured.
	// Repanic: true lets the existing panic recovery middleware handle the response
	// after Sentry has captured the event.
	var httpHandler http.Handler = r
	if cfg.Sentry.DSN != "" {
		httpHandler = sentryhttp.New(sentryhttp.Options{Repanic: true}).Handle(r)
		entry.Info("Sentry HTTP middleware active")
	}

	// WriteTimeout must exceed the longest per-route httpware.Timeout context.
	// The AI handler uses cfg.AI.TimeoutSec (default 30s); add a 15s buffer for
	// response serialisation and network flush. This prevents Caddy from seeing
	// premature closes while keeping the DoS surface small.
	writeTimeout := time.Duration(cfg.AI.TimeoutSec+15) * time.Second
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      httpHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: writeTimeout,
		IdleTimeout:  60 * time.Second,
	}

	return &AppResources{DB: db, Services: services, Server: srv, Limiter: limiter}, nil
}

// newLogger creates and configures a logger instance.
func newLogger(cfg *config.Config) *logrus.Logger {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		TimestampFormat:  "2006-01-02 15:04:05",
		FullTimestamp:    true,
		DisableColors:    true,
		QuoteEmptyFields: true,
	})

	level, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	return logger
}

// initDB establishes a connection to the database with configured pool settings.
func initDB(cfg *config.Config, logger *logrus.Logger) (*gorm.DB, error) {
	sqlDB, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(cfg.DBPool.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.DBPool.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.DBPool.ConnMaxLifetime) * time.Second)

	// Test the connection
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Open with GORM
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{
		Logger: gormlogger.New(
			logger.WithField("component", "db"),
			glogger.Warn,
			500*time.Millisecond,
			true,
		),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open GORM: %w", err)
	}

	return db, nil
}

// runMigrations applies database migrations using golang-migrate.
// It is only called when cfg.RunMigrations is true; the guard lives in bootstrap().
func runMigrations(cfg *config.Config, db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB: %w", err)
	}

	driver, err := migratepostgres.WithInstance(sqlDB, &migratepostgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migrate driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance("file://migrations", "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// jwksDiagKey mirrors the JWKS key structure for diagnostic logging only.
type jwksDiagKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
}

// pingSocrate checks that the Socrate JWKS endpoint is reachable at startup.
// It also parses the JWKS response to log every key's kty/use/alg/kid so that
// misconfigured keys (e.g. missing use:"sig") are immediately visible in logs.
func pingSocrate(cfg *config.Config, log *logrus.Entry) {
	if cfg.JWKS.URL == "" {
		log.Warn("Socrate JWKS URL is not configured — authentication will not work")
		return
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(cfg.JWKS.URL)
	if err != nil {
		log.WithError(err).Warnf("Socrate unreachable at %s — authentication will fail until it is reachable", cfg.JWKS.URL)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Warnf("Socrate JWKS probe returned HTTP %d — authentication may not work correctly", resp.StatusCode)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.WithError(err).Warn("failed to read JWKS response body")
		return
	}

	var jwks struct {
		Keys []jwksDiagKey `json:"keys"`
	}
	if err := json.Unmarshal(body, &jwks); err != nil {
		log.WithError(err).Warn("failed to parse JWKS JSON — raw JWKS may be invalid")
		return
	}

	usableCount := 0
	for _, k := range jwks.Keys {
		usable := k.Kty == "RSA" && k.Use == "sig"
		if usable {
			usableCount++
		}
		log.WithFields(logrus.Fields{
			"kty": k.Kty, "use": k.Use,
			"alg": k.Alg, "kid": k.Kid,
			"usable_by_jwtauth": usable,
		}).Info("JWKS key found")
	}

	if usableCount == 0 {
		log.Warn("⚠ No RSA sig keys found in JWKS — jwtauth will reject all tokens. " +
			"Check that Socrate JWKS keys have kty=RSA and use=sig")
	} else {
		log.Infof("Socrate reachable — %d usable RSA sig key(s) loaded from %s",
			usableCount, cfg.Socrate.BaseURL)
	}
}

// pingSocrateAdmin verifies that the Socrate admin API is reachable and that the
// configured app ID is valid. It does this by:
//  1. Exchanging client credentials for a service token (OAuth port).
//  2. Probing GET /api/apps/{app_id}/users on the admin port with that token.
//
// All failures are non-fatal: the server starts regardless, but clear warnings
// are emitted so misconfiguration is visible immediately in startup logs.
func pingSocrateAdmin(cfg *config.Config, log *logrus.Entry) {
	sc := cfg.Socrate
	if sc.AdminURL == "" {
		log.Warn("SOCRATE_ADMIN_URL is not set — service-account calls (invite emails, magic links) will not work")
		return
	}
	if sc.ClientID == "" || sc.ClientSecret == "" {
		log.Warn("SOCRATE_CLIENT_ID / SOCRATE_CLIENT_SECRET not set — service-account calls will not work")
		return
	}

	client := &http.Client{Timeout: 8 * time.Second}

	// ── Step 1: exchange client credentials for a service token ──────────────
	tokenURL := strings.TrimRight(sc.BaseURL, "/") + "/oauth/token"
	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {sc.ClientID},
		"client_secret": {sc.ClientSecret},
	}
	resp, err := client.Post(tokenURL, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		log.WithError(err).Warnf("Socrate admin probe: token exchange failed (POST %s) — invite emails will not work", tokenURL)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		log.Warnf("Socrate admin probe: token exchange returned HTTP %d — check SOCRATE_CLIENT_ID / SOCRATE_CLIENT_SECRET", resp.StatusCode)
		return
	}

	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tok); err != nil || tok.AccessToken == "" {
		log.Warn("Socrate admin probe: token exchange succeeded but response contained no access_token")
		return
	}
	log.Info("Socrate admin probe: service token obtained successfully")

	// ── Step 2: decode the JWT sub claim to get the authoritative app ID ─────
	// The sub claim ("app:<numeric_id>") is the ground truth — no manual config
	// needed. If SOCRATE_APP_ID is also set we cross-check and warn on mismatch.
	resolvedAppID := sc.AppID
	if parts := strings.Split(tok.AccessToken, "."); len(parts) == 3 {
		padded := parts[1]
		switch len(padded) % 4 {
		case 2:
			padded += "=="
		case 3:
			padded += "="
		}
		if payload, decErr := base64.URLEncoding.DecodeString(padded); decErr == nil {
			var claims struct {
				Sub string `json:"sub"`
			}
			if jsonErr := json.Unmarshal(payload, &claims); jsonErr == nil && claims.Sub != "" {
				tokenAppID := strings.TrimPrefix(claims.Sub, "app:")
				switch {
				case sc.AppID == "":
					resolvedAppID = tokenAppID
					log.WithField("app_id", resolvedAppID).Info("Socrate admin probe: app ID auto-resolved from service token sub claim")
				case tokenAppID == sc.AppID:
					log.Infof("Socrate admin probe: service token sub=%s matches SOCRATE_APP_ID=%s ✓", claims.Sub, sc.AppID)
				default:
					log.Warnf("Socrate admin probe: MISMATCH — service token sub=%s but SOCRATE_APP_ID=%s. "+
						"Remove SOCRATE_APP_ID from .env (it is now auto-resolved) or update it to %s.",
						claims.Sub, sc.AppID, tokenAppID)
					resolvedAppID = tokenAppID // trust the token over the config
				}
			}
		}
	}

	if resolvedAppID == "" {
		log.Warn("Socrate admin probe: could not determine app ID — skipping admin API connectivity check")
		return
	}

	// ── Step 3: check admin URL reachability with a read-only probe ──────────
	// Use GET /api/apps/{id}/service/users (list users) instead of POST so that
	// the probe never creates side-effects. A 200/206 means the route exists and
	// the app ID is accepted. A 401/403 means an auth or IP-allowlist issue.
	// A plain-text 404 means the route path does not exist in this Socrate version.
	probeURL := strings.TrimRight(sc.AdminURL, "/") + "/api/apps/" + resolvedAppID + "/service/users?per_page=1"
	req, _ := http.NewRequest(http.MethodGet, probeURL, nil)
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)

	resp2, err := client.Do(req)
	if err != nil {
		log.WithError(err).Warnf("Socrate admin probe: could not reach admin API at %s — invite emails will not work", sc.AdminURL)
		return
	}
	defer resp2.Body.Close()
	body2, _ := io.ReadAll(resp2.Body)
	bodyStr := strings.TrimSpace(string(body2))
	isJSON := strings.HasPrefix(bodyStr, "{") || strings.HasPrefix(bodyStr, "[")

	switch {
	case resp2.StatusCode == http.StatusOK || resp2.StatusCode == http.StatusPartialContent:
		log.Infof("Socrate admin API reachable and app ID %s accepted ✓", resolvedAppID)
	case resp2.StatusCode == http.StatusMethodNotAllowed:
		// 405 means the route exists and auth passed — Socrate just doesn't support GET
		// listing on the service/users endpoint (POST-only). Invite emails will work fine.
		log.Infof("Socrate admin API reachable and app ID %s accepted ✓ (probe got 405 — POST-only endpoint, expected)", resolvedAppID)
	case resp2.StatusCode == http.StatusNotFound && !isJSON:
		log.Warnf("Socrate admin probe: plain-text 404 from %s — GET /api/apps/{id}/service/users may not exist in this Socrate version. "+
			"Invite emails will likely fail at runtime.", probeURL)
	case resp2.StatusCode == http.StatusNotFound && isJSON:
		log.Warnf("Socrate admin probe: JSON 404 from %s (%s) — app ID %s may not exist in Socrate. "+
			"Verify SOCRATE_APP_ID matches the numeric database ID.", probeURL, bodyStr, resolvedAppID)
	case resp2.StatusCode == http.StatusUnauthorized || resp2.StatusCode == http.StatusForbidden:
		log.Warnf("Socrate admin probe: HTTP %d from %s (%s) — check IP allowlist and service token validity.",
			resp2.StatusCode, probeURL, bodyStr)
	default:
		log.Warnf("Socrate admin probe: unexpected HTTP %d from %s — %s", resp2.StatusCode, probeURL, bodyStr)
	}
}

// autoMigrate syncs all models to the database schema.
func autoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.Store{},
		&model.Employee{},
		&model.Contract{},
		&model.WeekTemplate{},
		&model.ShiftInstance{},
		&model.ShiftAssignment{},
		&model.CoverageRequirement{},
		&model.Availability{},
		&model.LeaveRequest{},
		&model.ShiftSlot{},
		&model.SwapRequest{},
		&model.AuditLog{},
		&model.Rule{},
		&model.AIInsight{},
		&model.SchedulePlan{},
		&model.Qualification{},
		&model.EmployeeQualification{},
		&model.PlanningModelMetric{},
		&model.PublicHoliday{},
		&model.StoreException{},
		&model.TokenRevocation{},
	)
}
