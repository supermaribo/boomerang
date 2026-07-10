package jobs

import (
	"fmt"

	"github.com/boomerang-backup/boomerang/internal/diskfree"
)

const minBackupHeadroom = 512 * 1024 * 1024 // 512 MiB

func checkDiskForBackup(dataDir string, estimateBytes int64, protocol string) error {
	free, ok := diskfree.Bytes(dataDir)
	if !ok {
		return nil
	}
	mult := 1.5
	if protocol == "rsync" {
		mult = 2.5
	}
	need := int64(float64(estimateBytes) * mult)
	if need < minBackupHeadroom {
		need = minBackupHeadroom
	}
	if int64(free) < need {
		return fmt.Errorf(
			"insufficient disk space: need at least %s free on the appliance (estimated %s for this backup), have %s",
			fmtBytes(need), fmtBytes(estimateBytes), fmtBytes(int64(free)),
		)
	}
	return nil
}

func fmtBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
