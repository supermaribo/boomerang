// Package metrics defines the wire format between boomerang-monitor and the
// central Boomerang poller.
package metrics

import "time"

const SchemaVersion = 1

// Sample is one point-in-time host snapshot.
type Sample struct {
	SchemaVersion  int          `json:"schemaVersion"`
	SampledAt      time.Time    `json:"sampledAt"`
	BootID         string       `json:"bootId,omitempty"`
	UptimeSec      int64        `json:"uptimeSec"`
	CPUPercent     float64      `json:"cpuPercent"`
	MemTotalBytes  uint64       `json:"memTotalBytes"`
	MemUsedBytes   uint64       `json:"memUsedBytes"`
	MemAvailBytes  uint64       `json:"memAvailBytes"`
	SwapTotalBytes uint64       `json:"swapTotalBytes"`
	SwapUsedBytes  uint64       `json:"swapUsedBytes"`
	Load1          float64      `json:"load1"`
	Load5          float64      `json:"load5"`
	Load15         float64      `json:"load15"`
	NumCPU         int          `json:"numCPU"`
	Filesystems    []Filesystem `json:"filesystems"`
	ClientVersion  string       `json:"clientVersion,omitempty"`
}

// Filesystem is usage for one local mount.
type Filesystem struct {
	Mount      string `json:"mount"`
	Device     string `json:"device,omitempty"`
	FSType     string `json:"fsType,omitempty"`
	TotalBytes uint64 `json:"totalBytes"`
	UsedBytes  uint64 `json:"usedBytes"`
	FreeBytes  uint64 `json:"freeBytes"`
}

// ExportBatch is returned by the restricted SSH export command.
type ExportBatch struct {
	SchemaVersion int      `json:"schemaVersion"`
	ClientVersion string   `json:"clientVersion"`
	Samples       []Sample `json:"samples"`
}

// LogSource is a read-only log stream exposed by the restricted monitor agent.
type LogSource struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
}
