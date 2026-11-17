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
	f.Add("0x0 (0000/)")
	f.Fuzz(func(t *testing.T, text string) {
		got, ok := ParseUnityErrorLog(text)
		if !ok {
			return
		}
		if got.Module == "" || strings.ContainsAny(got.Module, `/\`) {
			t.Fatalf("module is not a base name: %q", got.Module)
		}
		if IsSystemModule(got.Module) {
			t.Fatalf("named a system module %q", got.Module)
		}
	})
}

func FuzzParseCoredumpInfo(f *testing.F) {
	f.Add(coredumpInfo)
	f.Add("")
	f.Add("#0 0x0 (/ + 0x0)")
	f.Fuzz(func(t *testing.T, text string) {
		got, ok := ParseCoredumpInfo(text)
		if !ok {
			return
		}
		if got.Module == "" || strings.ContainsAny(got.Module, `/\`) {
			t.Fatalf("module is not a base name: %q", got.Module)
		}
		if IsSystemModule(got.Module) {
			t.Fatalf("named a system module %q", got.Module)
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
			if fault.Source != SourceEventLog {
				t.Fatalf("source = %q", fault.Source)
			}
			if len(fault.Managed) > maxManagedBytes {
				t.Fatalf("managed %d bytes", len(fault.Managed))
			}
		}
	})
}

func FuzzBaseName(f *testing.F) {
	f.Add(`C:\Game\Mods\Cool\Cool.dll`)
	f.Add(`/lib/x86_64-linux-gnu/libc.so.6`)
	f.Add(`Cool.dll`)
	f.Add(`0000/`)
	f.Add(`/`)
	f.Add("")
	f.Fuzz(func(t *testing.T, p string) {
		got := baseName(p)
		if strings.ContainsAny(got, `/\`) {
			t.Fatalf("baseName(%q) = %q still has a separator", p, got)
		}
		if !strings.ContainsAny(p, `/\`) && got != p {
			t.Fatalf("baseName(%q) = %q", p, got)
		}
	})
}
