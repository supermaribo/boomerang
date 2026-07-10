//go:build !linux

package hoststats

func readHost(dataDir string) Stats {
	_ = dataDir
	return Stats{}
}
