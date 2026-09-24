package state

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// A subscription is renewed at a check before it lapses, whatever the phase of the checks
// against the moment it was taken, so the controller never stops pushing changes.
func TestSubscriptionRenewedBeforeItLapses(t *testing.T) {
	req := require.New(t)
	taken := time.Date(2026, 9, 24, 3, 20, 26, 0, time.UTC)
	timeout := taken.Add(DefaultSubscriptionTimeout)

	for phase := time.Duration(0); phase < subscriptionCheck; phase += time.Second {
		check := taken.Add(phase)
		for !subscriptionDue(check, timeout) {
			check = check.Add(subscriptionCheck)
		}
		req.True(check.Before(timeout), "phase %v: renewed at %v, subscription lapsed at %v", phase, check, timeout)
		req.False(subscriptionDue(taken.Add(phase), timeout), "phase %v: renewed as soon as taken", phase)
	}
}
