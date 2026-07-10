package schedule

import (
	"fmt"
	"math/rand/v2"
	"time"
)

// Night backup hours (UTC): 23:00 through 06:00 inclusive.
var nightHours = []int{23, 0, 1, 2, 3, 4, 5, 6}

// RandomNight returns a daily cron and RFC3339 start time staggered across 23:00–06:00 UTC.
func RandomNight() (cron string, startRFC3339 string) {
	h := nightHours[rand.IntN(len(nightHours))]
	m := rand.IntN(60)
	cron = fmt.Sprintf("%d %d * * *", m, h)
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, time.UTC)
	return cron, start.Format(time.RFC3339)
}
