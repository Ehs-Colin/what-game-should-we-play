package steamapi

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ErrRateLimited is returned when Steam rejects a request with 429 or 403.
// The returned error is a *RateLimitError, which wraps ErrRateLimited.
var ErrRateLimited = errors.New("rate limited by Steam")

// ErrWishlistPrivate is returned when Steam rejects a wishlist request with
// 401 or 403. Unlike other endpoints, those statuses mean the wishlist is
// private rather than that the request was rate limited.
var ErrWishlistPrivate = errors.New("Steam wishlist is private")

// RateLimitError carries the Retry-After delay Steam sent, if any.
type RateLimitError struct {
	StatusCode int
	// RetryAfter is zero when Steam didn't send a usable Retry-After header.
	RetryAfter time.Duration
}

func (err *RateLimitError) Error() string {
	if err.RetryAfter > 0 {
		return fmt.Sprintf("%v (status %d, retry after %s)", ErrRateLimited, err.StatusCode, err.RetryAfter)
	}
	return fmt.Sprintf("%v (status %d)", ErrRateLimited, err.StatusCode)
}

func (err *RateLimitError) Unwrap() error {
	return ErrRateLimited
}

// checkStatus returns a *RateLimitError for 429/403 responses and a generic
// error for any other non-200 status.
func checkStatus(resp *http.Response, operation string) error {
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusTooManyRequests, http.StatusForbidden:
		return fmt.Errorf("%s: %w", operation, &RateLimitError{
			StatusCode: resp.StatusCode,
			RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"), time.Now()),
		})
	default:
		return fmt.Errorf("%s returned status %d", operation, resp.StatusCode)
	}
}

// checkWishlistStatus returns ErrWishlistPrivate for 401/403 responses, a
// *RateLimitError for 429 and a generic error for any other non-200 status.
func checkWishlistStatus(resp *http.Response, operation string) error {
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("%s: %w", operation, ErrWishlistPrivate)
	case http.StatusTooManyRequests:
		return fmt.Errorf("%s: %w", operation, &RateLimitError{
			StatusCode: resp.StatusCode,
			RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"), time.Now()),
		})
	default:
		return fmt.Errorf("%s returned status %d", operation, resp.StatusCode)
	}
}

// parseRetryAfter accepts either delay-seconds or an HTTP date.
func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds < 0 {
			return 0
		}
		return time.Duration(seconds) * time.Second
	}
	if date, err := http.ParseTime(value); err == nil {
		if delay := date.Sub(now); delay > 0 {
			return delay
		}
	}
	return 0
}
