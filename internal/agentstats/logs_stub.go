//go:build !linux

package agentstats

import (
	"fmt"

	"github.com/boomerang-backup/boomerang/internal/metrics"
)

func AvailableLogSources() []metrics.LogSource {
	return []metrics.LogSource{}
}

func ReadLogSource(lines int, source string) (string, error) {
	return "", fmt.Errorf("boomerang-monitor only supports Linux")
}
