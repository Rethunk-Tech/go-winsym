package crashdump

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Rethunk-Tech/go-winsym"
)

const (
	// maxDumpBytes bounds the dumps read: a full-memory dump is gigabytes, and only its header streams are needed, so
	// one larger than this is skipped rather than risked.
	maxDumpBytes = 4 << 30
	// maxLogBytes bounds a Unity error.log, which is a few KB; the cap keeps a damaged one from filling memory.
	maxLogBytes = 1 << 20
)

// Windows reads the dumps a crash since `since` left in a Windows user's profile: WER LocalDumps (crashDumps, named
// for the exe) and Unity crash folders under temp (<company>/<product>/Crashes/Crash_*). exes are the game's
// executable names; one that is not a bare file name is ignored. A Unity folder's error.log is read first, since it
// names the module itself.
func Windows(crashDumps, temp string, exes []string, since time.Time) []Fault {
	var out []Fault
	for _, dir := range unityCrashDirs(temp, since) {
		dump, haveDump := ReadDump(filepath.Join(dir, "crash.dmp"))
		if b, err := readLimited(dir, "error.log", maxLogBytes); err == nil {
			if f, ok := ParseUnityErrorLog(string(b)); ok {
				f.Path = dir
				f.Time = dirTime(dir)
				// The log names the module; the dump beside it has the offset and the module's debug record.
				if haveDump && strings.EqualFold(stem(dump.Module), stem(f.Module)) {
					f.Code, f.Offset, f.HasOffset, f.CodeView = dump.Code, dump.Offset, dump.HasOffset, dump.CodeView
				}
				out = append(out, f)
				continue
			}
		}
		if haveDump {
			out = append(out, dump)
		}
	}
	for _, exe := range exes {
		if exe == "" || filepath.Base(exe) != exe {
			continue
		}
		matches, _ := filepath.Glob(filepath.Join(crashDumps, stem(exe)+"*.dmp"))
		for _, path := range matches {
			if info, err := os.Stat(path); err != nil || !info.ModTime().After(since) {
				continue
			}
			if f, ok := ReadDump(path); ok {
				out = append(out, f)
			}
		}
	}
	return out
}

func unityCrashDirs(temp string, since time.Time) []string {
	matches, _ := filepath.Glob(filepath.Join(temp, "*", "*", "Crashes", "Crash_*"))
	var out []string
	for _, dir := range matches {
		if info, err := os.Stat(dir); err == nil && info.IsDir() && info.ModTime().After(since) {
			out = append(out, dir)
		}
	}
	return out
}

func dirTime(dir string) time.Time {
	if info, err := os.Stat(dir); err == nil {
		return info.ModTime()
	}
	return time.Time{}
}

// readLimited reads at most limit bytes of the file name inside dir. The file is opened through a root on dir, so a
// symlink or ".." in name cannot reach outside it.
func readLimited(dir, name string, limit int64) ([]byte, error) {
	f, err := os.OpenInRoot(dir, name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(io.LimitReader(f, limit))
}

// ReadDump reads the faulting module from the minidump at path. ok is false when the file is missing, larger than a
// dump is read at, or not a minidump with an exception and module list. The file is opened through a root on its own
// folder.
func ReadDump(path string) (Fault, bool) {
	f, err := os.OpenInRoot(filepath.Dir(path), filepath.Base(path))
	if err != nil {
		return Fault{}, false
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil || info.Size() > maxDumpBytes {
		return Fault{}, false
	}
	m, err := winsym.ParseMinidump(f, info.Size())
	if err != nil {
		return Fault{}, false
	}
	fault := fromMinidump(m)
	fault.Path, fault.Time = path, info.ModTime()
	return fault, true
}

func stem(name string) string { return strings.TrimSuffix(name, filepath.Ext(name)) }
