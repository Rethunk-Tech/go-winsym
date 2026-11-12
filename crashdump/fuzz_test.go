package crashdump

import (
	"os"
	"strings"
	"testing"
)

func FuzzParseUnityErrorLog(f *testing.F) {
	f.Add(unityLog)
	f.Add("")
	f.Add("nothing native here")
	f.Fuzz(func(t *testing.T, text string) {
		got, ok := ParseUnityErrorLog(text)
		if !ok {
			return
		}
		if got.Module == "" || strings.ContainsAny(got.Module, `/\`) {
			t.Fatalf("module is not a base name: %q", got.Module)
		}
	})
}

func FuzzParseCoredumpInfo(f *testing.F) {
	f.Add(coredumpInfo)
	f.Add("")
	f.Fuzz(func(t *testing.T, text string) {
		got, ok := ParseCoredumpInfo(text)
		if !ok {
			return
		}
		if got.Module == "" || strings.ContainsAny(got.Module, `/\`) {
			t.Fatalf("module is not a base name: %q", got.Module)
		}
	})
}

func FuzzParseEventLog(f *testing.F) {
	f.Add([]byte(eventsXML), "cat mail co.exe")
	f.Add([]byte("not xml \x00<<"), "a.exe")
	if raw, err := os.ReadFile("testdata/wevtutil-win11.xml"); err == nil {
		f.Add(raw, "CrashTest.exe")
	}
	f.Fuzz(func(t *testing.T, raw []byte, exe string) {
		got := ParseEventLog(raw, []string{exe})
		for _, fault := range got {
			if len(fault.Managed) > maxManagedBytes {
				t.Fatalf("managed %d bytes", len(fault.Managed))
			}
		}
	})
}
