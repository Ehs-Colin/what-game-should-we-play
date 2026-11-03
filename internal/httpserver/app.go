package httpserver

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/api"
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
}

type Application struct {
	Handler    http.Handler
	publicRoot *os.Root
}

func New(database *sql.DB, logger *logging.Logger, options Options) (*Application, error) {
	// renderer, err := templates.Load(options.TemplateDirectory)
	// if err != nil {
	// 	return nil, err
	// }
	publicRoot, err := os.OpenRoot(options.PublicDirectory)
	if err != nil {
		return nil, fmt.Errorf("open public directory: %w", err)
	}
	//TODO::Implement logger
	dynamicMux := http.NewServeMux()

	apiStore := api.NewStore(database, options.SteamApiKey, options.SteamClient)
	apiHandler := api.NewHandler(apiStore)
	//dynamicMux.HandleFunc("GET /api/steamuser/{id}", apiHandler.GetSteamUserById)
	//dynamicMux.HandleFunc("GET /api/steamuser/{name}", apiHandler.GetSteamUserByName)
	//dynamicMux.HandleFunc("GET /api/steamusers", apiHandler.GetSteamUsers)
	dynamicMux.HandleFunc("POST /api/steamuser", apiHandler.InsertSteamUser)

	// dynamicHandler := applyMiddleware(
	// 	dynamicMux,
	// 	LoadShedder(MaxInFlight, RetryAfterSeconds),
	// 	fixedWindowRateLimiter(rateLimitSettings),
	// 	validateRequestOrigin(options.AppOrigin, renderer),
	// )
	// mainMux := http.NewServeMux()
	// mainMux.HandleFunc("GET /health", func(responseWriter http.ResponseWriter, _ *http.Request) {
	// 	httpx.RespondWithJSON(responseWriter, http.StatusOK, map[string]any{"ok": true, "app": "bearly-secure"})
	// })
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
	// mainMux.Handle("/", dynamicHandler)

	// handler := applyMiddleware(
	// 	mainMux,
	// 	assignRequestId,
	// 	cspNonce,
	// 	securityHeaders,
	// 	recoverPanics(logger, renderer),
	// )

	return &Application{Handler: dynamicMux, publicRoot: publicRoot}, nil
}

func (application *Application) Close() error {
	return application.publicRoot.Close()
}
