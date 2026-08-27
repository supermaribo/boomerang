package remote

import (
	"net"
	"time"

	"github.com/boomerang-backup/boomerang/internal/tsdial"
	"github.com/jlaffaye/ftp"
)

func ftpDialOpts(ftps bool) []ftp.DialOption {
	opts := []ftp.DialOption{
		ftp.DialWithTimeout(15 * time.Second),
		ftp.DialWithDialFunc(func(network, address string) (net.Conn, error) {
			return tsdial.DialTimeout(network, address, 15*time.Second)
		}),
	}
	if ftps {
		opts = append(opts, ftp.DialWithExplicitTLS(nil))
	}
	return opts
}
