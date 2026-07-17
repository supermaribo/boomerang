package agentstats

import (
	"encoding/json"

	"github.com/boomerang-backup/boomerang/internal/metrics"
)

func encodeSample(s metrics.Sample) ([]byte, error) {
	return json.Marshal(s)
}

func decodeSample(b []byte) (metrics.Sample, error) {
	var s metrics.Sample
	err := json.Unmarshal(b, &s)
	return s, err
}
