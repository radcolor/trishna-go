package runtime

import "time"

var DefaultRetrySchedule = []time.Duration{
	10 * time.Second,
	30 * time.Second,
	1 * time.Minute,
	2 * time.Minute,
	5 * time.Minute,
	10 * time.Minute,
	15 * time.Minute,
	30 * time.Minute,
}

func RetryDelay(schedule []time.Duration, attempt int) (time.Duration, bool) {
	if attempt < 1 || attempt > len(schedule) {
		return 0, false
	}
	return schedule[attempt-1], true
}
