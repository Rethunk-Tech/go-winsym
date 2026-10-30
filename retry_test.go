package winsym

import (
	"errors"
	"testing"
)

// An error that is not another process holding the file is returned after one attempt, on every OS.
func TestRetryReturnsOtherErrorsAfterOneAttempt(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	calls := 0
	err := retry(func() error { calls++; return boom })
	if !errors.Is(err, boom) || calls != 1 {
		t.Fatalf("err = %v after %d calls", err, calls)
	}
	if err := retry(func() error { return nil }); err != nil {
		t.Fatal(err)
	}
}
