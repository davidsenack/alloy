//go:build !unix

package ledger

import "os"

// getOwnership returns 0, 0 on non-Unix systems where UID/GID aren't available.
func getOwnership(info os.FileInfo) (uid, gid uint32) {
	return 0, 0
}
