package bittorrent

import (
	"testing"

	"golang.org/x/time/rate"
)

func Test_newRateLimiter(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		ratePerSecond *uint64
		wantLimit     rate.Limit
	}{
		"nil": {
			ratePerSecond: nil,
			wantLimit:     rate.Inf,
		},
		"zero": {
			ratePerSecond: ptrTo(uint64(0)),
			wantLimit:     rate.Inf,
		},
		"bounded": {
			ratePerSecond: ptrTo(uint64(1024)),
			wantLimit:     rate.Limit(1024),
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			limiter := newRateLimiter(testCase.ratePerSecond, 1<<20)

			if limiter == nil {
				t.Fatal("newRateLimiter returned nil")
			}
			if got := limiter.Limit(); got != testCase.wantLimit {
				t.Errorf("expected limit %v, got %v", testCase.wantLimit, got)
			}
		})
	}
}
