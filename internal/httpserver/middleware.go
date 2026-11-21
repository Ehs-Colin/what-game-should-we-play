package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"sync"
	"time"
	"uuid"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/httpx"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/logging"
)

type middleware func(http.Handler) http.Handler

type contextKey string

const (
	requestIDContextKey contextKey = "request-id"
)

func applyMiddleware(handler http.Handler, middlewareChain ...middleware) http.Handler {
	for _, currentMiddleware := range slices.Backward(middlewareChain) {
		handler = currentMiddleware(handler)
	}
	return handler
}

func assignRequestId(next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		requestId := uuid.NewV4()
		responseWriter.Header().Set("X-Request-Id", requestId.String())

		ctx := context.WithValue(request.Context(), requestIDContextKey, requestId)
		next.ServeHTTP(responseWriter, request.WithContext(ctx))
	})
}

// used for failed logins
func getRequestId(ctx context.Context) uuid.UUID {
	requestId, ok := ctx.Value(requestIDContextKey).(uuid.UUID)
	if !ok {
		requestId = uuid.NewV4()
	}
	return requestId
}

func validateRequestOrigin(appOrigin string) middleware { //renderer *templates.Renderer
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			if request.Method != http.MethodPost {
				next.ServeHTTP(responseWriter, request)
				return
			}

			origin := request.Header.Get("Origin")
			if origin == appOrigin {
				next.ServeHTTP(responseWriter, request)
				return
			}
			if origin == "" {
				referer := request.Header.Get("Referer")
				parsedReferer, err := url.Parse(referer)
				if err == nil && referer != "" && parsedReferer.Scheme+"://"+parsedReferer.Host == appOrigin {
					next.ServeHTTP(responseWriter, request)
					return
				}
			}
			if err := httpx.RespondWithErrorPage(responseWriter, http.StatusForbidden, "Forbidden", "This request did not come from the what-game-should-we-play app."); err != nil {
				http.Error(responseWriter, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		})
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		allowedNonce := httpx.CSPNonce(request.Context())
		// Steam serves app header images and profile avatars from rotating
		// *.steamstatic.com CDN hosts, so the whole vendor subdomain tree is
		// allowed rather than pinning one host that changes over time.
		imageSources := "'self' data: https://*.steamstatic.com"
		responseWriter.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'nonce-"+allowedNonce+"'; style-src 'self'; img-src "+imageSources+"; frame-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'self'")
		responseWriter.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		responseWriter.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		responseWriter.Header().Set("Origin-Agent-Cluster", "?1")
		responseWriter.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		responseWriter.Header().Set("X-Content-Type-Options", "nosniff")
		responseWriter.Header().Set("X-DNS-Prefetch-Control", "off")
		responseWriter.Header().Set("X-Download-Options", "noopen")
		responseWriter.Header().Set("X-Frame-Options", "SAMEORIGIN")
		responseWriter.Header().Set("X-Permitted-Cross-Domain-Policies", "none")
		responseWriter.Header().Set("X-XSS-Protection", "0")
		next.ServeHTTP(responseWriter, request)
	})
}

func cspNonce(next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		nonceBytes := make([]byte, 16)
		if _, err := rand.Read(nonceBytes); err != nil {
			http.Error(responseWriter, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		nonce := base64.StdEncoding.EncodeToString(nonceBytes)
		request = request.WithContext(httpx.WithCSPNonce(request.Context(), nonce))
		next.ServeHTTP(responseWriter, request)
	})
}

// recoverPanics turns a handler panic into a logged 500 error page. The panic
// value is logged but never shown to the client.
func recoverPanics(logger *logging.Logger) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			defer func() {
				recovered := recover()
				if recovered == nil {
					return
				}
				// http.ErrAbortHandler is the documented way for a handler to
				// abort a response, so let the server handle it.
				if recovered == http.ErrAbortHandler {
					panic(recovered)
				}
				if logger != nil {
					_ = logger.Event("unhandled_error", map[string]any{
						"requestId": getRequestId(request.Context()).String(),
						"method":    request.Method,
						"path":      request.URL.Path,
						"message":   fmt.Sprint(recovered),
					})
				}
				if err := httpx.RespondWithErrorPage(responseWriter, http.StatusInternalServerError, "Unhandled Error", "The request failed. Try again."); err != nil {
					http.Error(responseWriter, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(responseWriter, request)
		})
	}
}

func LoadShedder(maxConcurrent int, retryAfterSeconds int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if maxConcurrent <= 0 {
			panic("load-shedder: in-flight limit must be positive")
		}
		if retryAfterSeconds <= 0 {
			panic("load-shedder: retry delay must be positive")
		}

		capacity := make(chan struct{}, maxConcurrent)
		return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			responseWriter.Header().Set("X-In-Flight-Limit", strconv.Itoa(maxConcurrent))
			select {
			case capacity <- struct{}{}:
				defer func() { <-capacity }()
				next.ServeHTTP(responseWriter, request)
			default:
				responseWriter.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
				httpx.RespondWithJSON(responseWriter, http.StatusServiceUnavailable, map[string]string{"error": "Service is at capacity."})
			}
		})
	}
}

// SearchThrottle limits search requests to protect the Steam API budget.
func SearchThrottle() func(http.Handler) http.Handler {
	return fixedWindowRateLimiter(rateLimitOptions{
		window:  time.Second,
		maximum: 5,
		key: func(r *http.Request) string {
			return "search"
		},
		onLimit: func(w http.ResponseWriter, r *http.Request, state rateLimitState) {
			if err := httpx.RespondWithErrorPage(w, http.StatusTooManyRequests, "Search Is Busy", "Try again shortly."); err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		},
	})
}

type rateLimitCounter struct {
	count   int
	resetAt time.Time
}

type rateLimitOptions struct {
	window  time.Duration
	maximum int
	key     func(*http.Request) string
	onLimit func(http.ResponseWriter, *http.Request, rateLimitState)
	now     func() time.Time
}

type rateLimitState struct {
	limit             int
	remaining         int
	resetAt           time.Time
	retryAfterSeconds int
}

type fixedWindowLimiter struct {
	options       rateLimitOptions
	counters      map[string]rateLimitCounter
	countersMutex sync.Mutex
	nextSweepAt   time.Time
}

func newFixedWindowLimiter(options rateLimitOptions) *fixedWindowLimiter {
	validateRateLimitOptions(options)
	if options.now == nil {
		options.now = time.Now
	}
	if options.key == nil {
		options.key = httpx.ClientIPKey
	}
	return &fixedWindowLimiter{
		options:     options,
		counters:    make(map[string]rateLimitCounter),
		nextSweepAt: options.now().Add(options.window),
	}
}

func validateRateLimitOptions(options rateLimitOptions) {
	if options.window <= 0 {
		panic("rate-limit window must be positive")
	}
	if options.maximum <= 0 {
		panic("rate-limit maximum must be positive")
	}
}

func (limiter *fixedWindowLimiter) consume(request *http.Request) (rateLimitState, bool) {
	now := limiter.options.now()
	key := limiter.options.key(request)

	limiter.countersMutex.Lock()
	defer limiter.countersMutex.Unlock()
	limiter.sweepExpiredCounters(now)

	counter, exists := limiter.counters[key]
	if !exists || !now.Before(counter.resetAt) {
		counter = rateLimitCounter{resetAt: now.Add(limiter.options.window)}
	}
	retryAfterSeconds := max(1, int(math.Ceil(counter.resetAt.Sub(now).Seconds())))
	if counter.count >= limiter.options.maximum {
		return rateLimitState{
			limit:             limiter.options.maximum,
			remaining:         0,
			resetAt:           counter.resetAt,
			retryAfterSeconds: retryAfterSeconds,
		}, true
	}

	counter.count++
	limiter.counters[key] = counter
	return rateLimitState{
		limit:             limiter.options.maximum,
		remaining:         limiter.options.maximum - counter.count,
		resetAt:           counter.resetAt,
		retryAfterSeconds: retryAfterSeconds,
	}, false
}

func (limiter *fixedWindowLimiter) sweepExpiredCounters(now time.Time) {
	if now.Before(limiter.nextSweepAt) {
		return
	}
	for counterKey, counter := range limiter.counters {
		if !now.Before(counter.resetAt) {
			delete(limiter.counters, counterKey)
		}
	}
	limiter.nextSweepAt = now.Add(limiter.options.window)
}

func (limiter *fixedWindowLimiter) reject(responseWriter http.ResponseWriter, request *http.Request, state rateLimitState) {
	setRateLimitHeaders(responseWriter, state)
	responseWriter.Header().Set("Retry-After", strconv.Itoa(state.retryAfterSeconds))
	if limiter.options.onLimit != nil {
		limiter.options.onLimit(responseWriter, request, state)
		return
	}
	httpx.RespondWithJSON(responseWriter, http.StatusTooManyRequests, map[string]string{"error": "Too many requests"})
}

func fixedWindowRateLimiter(options rateLimitOptions) middleware {
	fixedWindowLimiter := newFixedWindowLimiter(options)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			state, limited := fixedWindowLimiter.consume(request)
			if limited {
				fixedWindowLimiter.reject(responseWriter, request, state)
				return
			}
			setRateLimitHeaders(responseWriter, state)
			next.ServeHTTP(responseWriter, request)
		})
	}
}

func setRateLimitHeaders(responseWriter http.ResponseWriter, state rateLimitState) {
	responseWriter.Header().Set("RateLimit-Limit", strconv.Itoa(state.limit))
	responseWriter.Header().Set("RateLimit-Remaining", strconv.Itoa(state.remaining))
	responseWriter.Header().Set("RateLimit-Reset", strconv.FormatInt(state.resetAt.Unix()+boolToInt64(state.resetAt.Nanosecond() > 0), 10))
}

func boolToInt64(value bool) int64 {
	if value {
		return 1
	}
	return 0
}
