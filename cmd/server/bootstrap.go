package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	sentry "github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
	"github.com/golang-migrate/migrate/v4"
	migratepostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"

	"github.com/ovander/backendkit/bff"
	"github.com/ovander/backendkit/gormlogger"
	"github.com/ovander/backendkit/httpware"
	"github.com/ovander/backendkit/jwtauth"

	"github.com/ovander/parashift/internal/config"
	"github.com/ovander/parashift/internal/handler"
	"github.com/ovander/parashift/internal/middleware"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/pkg/tracing"
	"github.com/ovander/parashift/internal/repo"
	"github.com/ovander/parashift/internal/router"
	"github.com/ovander/parashift/internal/service"
)

// AppResources holds all live application resources for clean shutdown.
type AppResources struct {
	DB             *gorm.DB
	Services       *service.ServiceBundle
	Server         *http.Server
	Limiter        *httpware.RateLimiter
	TracerShutdown func(context.Context) error
	StopBFF        func() // stops the BFF session sweeper
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

	// Step 2.6: OpenTelemetry tracing (OBS-3). No-op when OTEL_EXPORTER_OTLP_ENDPOINT
	// is unset; otherwise installs the global tracer provider + propagator.
	tracerShutdown, err := tracing.Init(context.Background(), tracing.Config{
		Endpoint:       cfg.Tracing.Endpoint,
		Insecure:       cfg.Tracing.Insecure,
		SampleRatio:    cfg.Tracing.SampleRatio,
		ServiceName:    "parashift-backend",
		ServiceVersion: build.Version,
		Environment:    cfg.Env,
	})
	if err != nil {
		entry.WithError(err).Warn("tracing init failed — continuing without tracing")
		tracerShutdown = func(context.Context) error { return nil }
	} else if cfg.Tracing.Endpoint != "" {
		entry.WithField("endpoint", cfg.Tracing.Endpoint).Info("OpenTelemetry tracing enabled")
	}

	// Step 3: Socrate connectivity check (the one documented hand-written
	// request to Socrate: a GET of its public JWKS). Every other call goes
	// through backendkit's socrate.Client.
	pingSocrate(cfg, entry)
	logSocrateAdmin(cfg, entry)

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
	repos := repo.NewRepoBundle(db)
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
	var profiles middleware.ProfileReader // nil interface when Socrate is not configured
	if services.SocrateClient != nil {
		profiles = services.SocrateClient
	}
	tenantMW := middleware.NewTenantMiddleware(repos.Employee, logger.WithField("component", "tenant"), profiles)
	rbacMW := middleware.NewRBACMiddleware(logger.WithField("component", "rbac"))
	limiter := httpware.NewRateLimiter(100, 200)

	bffRoutes, stopBFF := newBFF(cfg, services, logger.WithField("component", "bff"))

	mw := router.Middleware{
		// jwtauth validates the token; AppRole then replaces its role with
		// Parashift's own (app_roles[SOCRATE_CLIENT_ID]), never the top-level claim.
		Auth: func(next http.Handler) http.Handler {
			return jwtMW.Handler(middleware.AppRole(cfg.Socrate.ClientID)(next))
		},
		Tenant:         tenantMW,
		RBAC:           rbacMW,
		GeneralLimiter: limiter,
		Logger:         logger, // *logrus.Logger for httpware.Logger
		BFF:            bffRoutes,
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
		Addr:         cfg.ListenAddr(),
		Handler:      httpHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: writeTimeout,
		IdleTimeout:  60 * time.Second,
	}

	return &AppResources{DB: db, Services: services, Server: srv, Limiter: limiter, TracerShutdown: tracerShutdown, StopBFF: stopBFF}, nil
}

// bffSweepInterval is how often expired BFF sessions and pending sign-ins are
// dropped from memory.
const bffSweepInterval = time.Minute

// newBFF builds the Backend-for-Frontend: an in-memory session store (idle and
// absolute lifetimes from BFF_SESSION_*_TTL), the session cookie, the /bff
// routes, the session middleware in front of /api/v1, and a ticker that sweeps
// expired sessions. The store is per process: one instance, and a restart
// signs everyone out.
//
// It returns nil (the API then takes bearer tokens only, as before) when
// BFF_REDIRECT_URL is unset, which Validate allows outside production only.
// During the transition to the BFF, /api/v1 still accepts a bearer without a
// session, so the current SPA keeps working.
func newBFF(cfg *config.Config, services *service.ServiceBundle, log *logrus.Entry) (*router.BFF, func()) {
	if !cfg.BFF.Enabled() {
		log.Info("BFF disabled: BFF_REDIRECT_URL not set; the API takes bearer tokens only")
		return nil, func() {}
	}
	if services.SocrateClient == nil || services.SessionAuth == nil {
		log.Warn("BFF disabled: the Socrate client is not configured")
		return nil, func() {}
	}
	store := bff.NewMemoryStore(cfg.BFF.IdleTTL, cfg.BFF.AbsoluteTTL)
	gw := &bff.Gateway{
		Store: store,
		Cookie: bff.CookieConfig{
			Name:   cfg.BFF.CookieName,
			Secure: !cfg.BFF.InsecureCookie, // Validate allows false only over http outside production
			MaxAge: int(cfg.BFF.AbsoluteTTL.Seconds()),
		},
		Refresher: middleware.EndWithoutRefreshToken(services.SocrateClient),
	}
	h := handler.NewBFFHandler(handler.BFFOptions{
		Gateway:     gw,
		Auth:        services.SessionAuth,
		Issuer:      cfg.Socrate.BaseURL,
		ClientID:    cfg.Socrate.ClientID,
		RedirectURI: cfg.BFF.RedirectURL,
		Logger:      log,
	})
	stop := make(chan struct{})
	go func() {
		t := time.NewTicker(bffSweepInterval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				store.Sweep()
				h.Sweep()
			case <-stop:
				return
			}
		}
	}()
	log.WithFields(logrus.Fields{
		"cookie":       gw.Cookie.CookieName(),
		"idle_ttl":     cfg.BFF.IdleTTL.String(),
		"absolute_ttl": cfg.BFF.AbsoluteTTL.String(),
		"redirect_uri": cfg.BFF.RedirectURL,
	}).Info("BFF enabled: /bff routes; /api/v1 takes a session or, during the transition, a bearer")
	var once sync.Once
	return &router.BFF{Handler: h, Session: middleware.NewSessionAuth(gw, true, log)},
		func() { once.Do(func() { close(stop) }) }
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

// logSocrateAdmin records how service-account calls (invite e-mails, magic
// links) will reach Socrate. It sends nothing: the admin API is reached only
// through socrate.Client, and a missing setting is reported here instead of
// at the first invitation.
func logSocrateAdmin(cfg *config.Config, log *logrus.Entry) {
	sc := cfg.Socrate
	switch {
	case sc.ClientID == "" || sc.ClientSecret == "":
		log.Warn("SOCRATE_CLIENT_ID / SOCRATE_CLIENT_SECRET not set — service-account calls will not work")
	case sc.AdminURL == "":
		log.Warn("SOCRATE_ADMIN_URL is not set — service-account calls (invite emails, magic links) will not work")
	case sc.AppID == "":
		log.Warn("SOCRATE_APP_ID is not set — service-account calls (invite emails, magic links) will not work")
	default:
		log.WithFields(logrus.Fields{"admin_url": sc.AdminURL, "app_id": sc.AppID}).
			Info("Socrate service-account calls configured")
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
