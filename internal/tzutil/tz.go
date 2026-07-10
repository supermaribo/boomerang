package tzutil

import (
	"fmt"
	"strings"
	"time"

	"github.com/boomerang-backup/boomerang/internal/store"
)

const MetaKey = "appliance_timezone"

// Common IANA zones offered in the UI (any valid IANA name is accepted).
var Common = []string{
	"UTC",
	"Europe/London",
	"Europe/Dublin",
	"Europe/Paris",
	"Europe/Berlin",
	"Europe/Amsterdam",
	"America/New_York",
	"America/Chicago",
	"America/Denver",
	"America/Los_Angeles",
	"America/Toronto",
	"America/Vancouver",
	"Australia/Sydney",
	"Australia/Melbourne",
	"Pacific/Auckland",
	"Asia/Singapore",
	"Asia/Tokyo",
	"Asia/Dubai",
}

func Normalize(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "UTC"
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return "", fmt.Errorf("unknown timezone %q", name)
	}
	return loc.String(), nil
}

func Load(st *store.Store) *time.Location {
	if st == nil {
		return time.UTC
	}
	name, _, _ := st.GetMeta(MetaKey)
	if name == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}

func Name(st *store.Store) string {
	if st == nil {
		return "UTC"
	}
	name, _, _ := st.GetMeta(MetaKey)
	if name == "" {
		return "UTC"
	}
	if _, err := time.LoadLocation(name); err != nil {
		return "UTC"
	}
	return name
}

func Format(st *store.Store, t time.Time) string {
	return t.In(Load(st)).Format("2 Jan 2006, 15:04 MST")
}
