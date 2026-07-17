//go:build !linux

package agentstats

import "fmt"

func ReadJournal(lines int, unit string) (string, error) {
	return "", fmt.Errorf("boomerang-monitor only supports Linux")
}
