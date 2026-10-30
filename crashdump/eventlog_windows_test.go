package crashdump

import (
	"testing"
	"time"
)

// The Application log is the real one: an exe name no program has, so the answer is empty whatever the machine ran.
func TestEventLogRunsWevtutilAndFindsNoFaultForAnUnknownExe(t *testing.T) {
	t.Parallel()
	if got := EventLog(t.Context(), []string{"winsym-no-such-program.exe"}, time.Now().Add(-time.Hour)); len(got) != 0 {
		t.Fatalf("faults = %+v", got)
	}
}
