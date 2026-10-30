package winsym

import (
	"fmt"
	"syscall"
	"testing"
)

func TestRetryOutlastsAFileAnotherProcessHolds(t *testing.T) {
	t.Parallel()
	for _, errno := range []syscall.Errno{errAccessDenied, errSharingViolation, errLockViolation} {
		calls := 0
		err := retry(func() error {
			calls++
			if calls < 4 {
				return fmt.Errorf("rename: %w", errno)
			}
			return nil
		})
		if err != nil || calls != 4 {
			t.Errorf("errno %d: err = %v after %d calls", errno, err, calls)
		}
	}
}

func TestRetryGivesUpOnAFileHeldForGood(t *testing.T) {
	t.Parallel()
	calls := 0
	err := retry(func() error { calls++; return errSharingViolation })
	if err == nil || calls < 5 {
		t.Fatalf("err = %v after %d calls", err, calls)
	}
}
