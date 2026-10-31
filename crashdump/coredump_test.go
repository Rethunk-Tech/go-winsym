package crashdump

import (
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestCoredump(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in program is a shell script")
	}
	since := time.Now()
	t.Setenv("PATH", t.TempDir())
	if _, ok := Coredump(t.Context(), "Game", since); ok {
		t.Fatal("coredumpctl is not installed")
	}
	// The script insists on the argv Coredump promises and prints a canned `coredumpctl info`.
	bin := fakeBin(t, "coredumpctl", `[ "$1" = info ] && [ "$2" = --no-pager ] && [ "$3" = --since ] && [ "$5" = -- ] && [ "$6" = Game ] || exit 1
printf '%s' '`+coredumpInfo+`'
`)
	t.Setenv("PATH", filepath.Dir(bin))
	f, ok := Coredump(t.Context(), "Game", since)
	if !ok || f.Module != "libCoolNative.so" || f.Source != SourceCoredump {
		t.Fatalf("fault = %+v, %v", f, ok)
	}
	if _, ok := Coredump(t.Context(), "Other", since); ok {
		t.Fatal("a refusing coredumpctl gave a fault")
	}
}

func TestIsSystemModule(t *testing.T) {
	for name, want := range map[string]bool{"KERNEL32.dll": true, "ntdll": true, "libc.so.6": true, "CoolMod.dll": false} {
		if IsSystemModule(name) != want {
			t.Errorf("%s: want %v", name, want)
		}
	}
}
