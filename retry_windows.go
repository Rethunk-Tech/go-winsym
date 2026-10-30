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

// retry runs op again, doubling the wait from 1 ms, while it fails because another process holds the file open and
// the next wait would still end within two seconds. Defender or the search indexer scans a file just written; renaming
// or deleting it then fails until they let go, and giving up would waste the download and repeat it on the next call.
func retry(op func() error) error {
	start := time.Now()
	for wait := time.Millisecond; ; wait *= 2 {
		err := op()
		if !held(err) || time.Since(start)+wait > 2*time.Second {
			return err
		}
		time.Sleep(wait)
	}
}

func held(err error) bool {
	var errno syscall.Errno
	return errors.As(err, &errno) && (errno == errAccessDenied || errno == errSharingViolation || errno == errLockViolation)
}
