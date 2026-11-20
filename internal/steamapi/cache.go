package steamapi

import (
	"context"
	"math"
	"time"
)

const (
	CacheMaxTTL      = 24 * time.Hour
	CacheMinTTL      = time.Hour
	CacheWindow      = 30 * 24 * time.Hour
	CacheCurveMid    = 45.0
	CacheCurveWidth  = 10.0
	CacheCurveTarget = 90
)

// Cache stores raw Steam API response bodies keyed by normalized resource.
type Cache interface {
	// Load returns the cached body and whether it is still fresh at now.
	// A missing key returns found=false and a nil error.
	Load(ctx context.Context, key string, now time.Time) (body []byte, fresh, found bool, err error)
	// Record stores body and records a successful fetch at now.
	Record(ctx context.Context, key string, body []byte, now time.Time) error
}

// CacheTTL returns the lifetime for a cached response after the given number
// of successful fetches in the sliding window. A non-positive count returns
// the maximum.
func CacheTTL(calls int) time.Duration {
	if calls <= 0 {
		return CacheMaxTTL
	}
	logistic := func(n float64) float64 {
		return 1 / (1 + math.Exp((n-CacheCurveMid)/CacheCurveWidth))
	}
	low := logistic(CacheCurveTarget)
	high := logistic(0)
	minHours := CacheMinTTL.Hours()
	maxHours := CacheMaxTTL.Hours()
	hours := minHours + (maxHours-minHours)*(logistic(float64(calls))-low)/(high-low)
	if hours < minHours {
		hours = minHours
	}
	if hours > maxHours {
		hours = maxHours
	}
	return time.Duration(math.Round(hours*3600)) * time.Second
}
