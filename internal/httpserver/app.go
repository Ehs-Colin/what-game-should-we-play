package httpserver

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/api"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/httpx"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/importer"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/logging"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/steamapi"
)

const (
	MaxInFlight       = 50
	RetryAfterSeconds = 1
)

type Options struct {
	AppOrigin        string
	PublicDirectory  string
	TrustedProxyHops int
	SteamApiKey      string
	SteamClient      *steamapi.Client
	Importer         *importer.Importer
	Auth             api.AuthService
	Sessions         SessionResolver
}

type Application struct {
	Handler    http.Handler
	publicRoot *os.Root
}

func New(database *sql.DB, logger *logging.Logger, options Options) (*Application, error) {
	publicRoot, err := os.OpenRoot(options.PublicDirectory)
	if err != nil {
		return nil, fmt.Errorf("open public directory: %w", err)
	}
	//TODO::Implement logger
	dynamicMux := http.NewServeMux()

	apiStore := api.NewStore(database, options.SteamApiKey, options.SteamClient)
	loginLimiter := api.NewLoginThrottle()
	apiHandler := api.NewHandler(
		apiStore,
		options.Importer,
		logger,
		options.Auth,
		strings.HasPrefix(options.AppOrigin, "https://"),
		loginLimiter,
		httpx.ClientIPKeyWithTrustedProxies(options.TrustedProxyHops),
	)

	dynamicMux.HandleFunc("POST /api/auth/login", apiHandler.Login)
	dynamicMux.HandleFunc("POST /api/auth/logout", apiHandler.Logout)
	dynamicMux.HandleFunc("GET /api/auth/session", apiHandler.Session)
	dynamicMux.HandleFunc("POST /api/auth/account", apiHandler.CreateAccount)

	dynamicMux.HandleFunc("GET /api/dashboard", apiHandler.GetDashboard)
	dynamicMux.HandleFunc("POST /api/steamuser", apiHandler.InsertSteamUser)
	dynamicMux.HandleFunc("GET /api/steamusers", apiHandler.GetSteamUserByName)
	dynamicMux.HandleFunc("GET /api/steamusers/search", apiHandler.SearchSteamUsers)
	dynamicMux.HandleFunc("GET /api/steamusers/by-ids", apiHandler.GetSteamUsersByIDs)
	dynamicMux.HandleFunc("GET /api/steamusers/by-name", apiHandler.GetSteamUserView)
	dynamicMux.HandleFunc("GET /api/import-jobs/{id}", apiHandler.GetImportJob)
	dynamicMux.HandleFunc("POST /api/common-games", apiHandler.GetCommonGames)

	// Serve the static front end. The API patterns above are more specific, so
	// they take precedence over this catch-all. Unknown extensionless paths
	// fall back to index.html so client-side routes survive a hard refresh.
	dynamicMux.Handle("GET /", newSPAHandler(publicRoot))

	rateLimitSettings := rateLimitOptions{
		window:  time.Minute,
		maximum: 100,
		key:     httpx.ClientIPKeyWithTrustedProxies(options.TrustedProxyHops),
	}
	dynamicHandler := applyMiddleware(
		dynamicMux,
		LoadShedder(MaxInFlight, RetryAfterSeconds),
		fixedWindowRateLimiter(rateLimitSettings),
		validateRequestOrigin(options.AppOrigin),
		requireAuth(options.Sessions, logger),
	)

	mainMux := http.NewServeMux()
	mainMux.HandleFunc("GET /health", func(responseWriter http.ResponseWriter, _ *http.Request) {
		httpx.RespondWithJSON(responseWriter, http.StatusOK, map[string]any{"ok": true, "app": "what-game-should-we-play"})
	})
	// staticHandler := newStaticHandler(publicRoot)
	// mainMux.Handle("GET /reset.css", staticHandler)
	// mainMux.Handle("GET /styles.css", staticHandler)
	// mainMux.Handle("GET /passkey.js", staticHandler)
	// mainMux.Handle("GET /vendor/simplewebauthn/index.umd.min.js", staticHandler)
	// mainMux.Handle("GET /shipping-widget.css", staticHandler)
	// mainMux.Handle("GET /shipping-widget.html", staticHandler)
	// mainMux.Handle("GET /shipping-widget.js", staticHandler)
	// mainMux.Handle("GET /product-photos/{filename}", staticHandler)
	// mainMux.HandleFunc("POST /integrations/pawpal/webhook", pawPalHandler.Webhook)
	mainMux.Handle("/", dynamicHandler)

	handler := applyMiddleware(
		mainMux,
		assignRequestId,
		cspNonce,
		securityHeaders,
		recoverPanics(logger),
	)

	return &Application{Handler: handler, publicRoot: publicRoot}, nil
}

func newSPAHandler(publicRoot *os.Root) http.Handler {
	fileServer := http.FileServer(http.FS(publicRoot.FS()))
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/api" || strings.HasPrefix(request.URL.Path, "/api/") {
			http.NotFound(responseWriter, request)
			return
		}

		name := strings.TrimPrefix(path.Clean(request.URL.Path), "/")
		if name != "" && name != "." {
			if info, err := publicRoot.Stat(name); err == nil && !info.IsDir() {
				fileServer.ServeHTTP(responseWriter, request)
				return
			}
			if path.Ext(name) != "" {
				http.NotFound(responseWriter, request)
				return
			}
		}

		// Extensionless paths that do not exist are client-side routes. Serve
		// the SPA shell instead of masking a missing asset with HTML.
		clone := request.Clone(request.Context())
		clone.URL.Path = "/"
		fileServer.ServeHTTP(responseWriter, clone)
	})
}

func (application *Application) Close() error {
	return application.publicRoot.Close()
}
