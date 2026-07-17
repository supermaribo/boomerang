//go:build !linux

package agentstats

import (
	"fmt"
	"time"

	"github.com/boomerang-backup/boomerang/internal/metrics"
)

const DefaultSpoolDir = "/var/lib/boomerang-monitor/spool"

func Collect(clientVersion string) (metrics.Sample, error) {
	return metrics.Sample{}, fmt.Errorf("boomerang-monitor only supports Linux")
}

func AppendSample(spoolDir string, sample metrics.Sample) error {
	return fmt.Errorf("boomerang-monitor only supports Linux")
}

func ReadSince(spoolDir string, since time.Time) ([]metrics.Sample, error) {
	return nil, fmt.Errorf("boomerang-monitor only supports Linux")
}
