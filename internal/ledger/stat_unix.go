//go:build unix

package ledger

import (
	"os"
	"syscall"
)

// getOwnership extracts UID and GID from file info on Unix systems.
func getOwnership(info os.FileInfo) (uid, gid uint32) {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return stat.Uid, stat.Gid
	}
	return 0, 0
}
