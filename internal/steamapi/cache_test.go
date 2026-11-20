package steamapi

import (
	"testing"
	"time"
)

func TestCacheTTLNonPositiveIsMaximum(t *testing.T) {
	for _, calls := range []int{-10, -1, 0} {
		if got := CacheTTL(calls); got != CacheMaxTTL {
			t.Errorf("CacheTTL(%d) = %v, want %v", calls, got, CacheMaxTTL)
		}
	}
}

func TestCacheTTLTable(t *testing.T) {
	tests := []struct {
		calls int
		want  time.Duration
	}{
		{calls: 1, want: 23*time.Hour + 58*time.Minute + 23*time.Second},
		{calls: 10, want: 23*time.Hour + 34*time.Minute + 9*time.Second},
		{calls: 50, want: 9*time.Hour + 37*time.Minute + 13*time.Second},
		{calls: 90, want: CacheMinTTL},
	}
	for _, test := range tests {
		got := CacheTTL(test.calls)
		diff := got - test.want
		if diff < 0 {
			diff = -diff
		}
		if diff > time.Second {
			t.Errorf("CacheTTL(%d) = %v, want %v (within 1s)", test.calls, got, test.want)
		}
	}
}

func TestCacheTTLClampsToMinimum(t *testing.T) {
	for _, calls := range []int{90, 91, 100, 1000} {
		if got := CacheTTL(calls); got != CacheMinTTL {
			t.Errorf("CacheTTL(%d) = %v, want %v", calls, got, CacheMinTTL)
		}
	}
}

func TestCacheTTLIsNonIncreasing(t *testing.T) {
	previous := CacheTTL(0)
	for calls := 1; calls <= 200; calls++ {
		got := CacheTTL(calls)
		if got > previous {
			t.Fatalf("CacheTTL(%d) = %v is greater than CacheTTL(%d) = %v", calls, got, calls-1, previous)
		}
		previous = got
	}
}
