package winsym

import (
	"errors"
	"syscall"
	"time"
)

const (
	errAccessDenied     syscall.Errno = 5  // ERROR_ACCESS_DENIED
	errSharingViolation syscall.Errno = 32 // ERROR_SHARING_VIOLATION
	errLockViolation    syscall.Errno = 33 // ERROR_LOCK_VIOLATION
)

// retry runs op again for about two seconds, doubling the wait from 1 ms, while it fails because another process holds
// the file open. Defender or the search indexer scans a file just written; renaming or deleting it then fails until
// they let go, and giving up would waste the download and repeat it on the next call.
func retry(op func() error) error {
	wait := time.Millisecond
	var err error
	for elapsed := time.Duration(0); ; elapsed += wait {
		if err = op(); !held(err) || elapsed >= 2*time.Second {
			return err
		}
		time.Sleep(wait)
		wait *= 2
	}
}

func held(err error) bool {
	var errno syscall.Errno
	return errors.As(err, &errno) && (errno == errAccessDenied || errno == errSharingViolation || errno == errLockViolation)
}
