package ai

import "time"

const (
	MinimumConfigurableTimeout = 5 * time.Minute
	LegacyTotalRequestTimeout  = 120 * time.Second
)

func EffectiveTimeout(timeout time.Duration) time.Duration {
	if timeout < MinimumConfigurableTimeout {
		return MinimumConfigurableTimeout
	}
	return timeout
}

func EffectiveTimeoutFromSeconds(seconds int) time.Duration {
	if seconds <= 0 {
		return MinimumConfigurableTimeout
	}
	return EffectiveTimeout(time.Duration(seconds) * time.Second)
}

func TimeoutSeconds(timeout time.Duration) int {
	return int(EffectiveTimeout(timeout).Seconds())
}

func MaxTimeout(timeouts ...time.Duration) time.Duration {
	maxTimeout := MinimumConfigurableTimeout
	for _, timeout := range timeouts {
		effective := EffectiveTimeout(timeout)
		if effective > maxTimeout {
			maxTimeout = effective
		}
	}
	return maxTimeout
}
