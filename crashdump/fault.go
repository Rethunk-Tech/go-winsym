package crashdump

import (
	"fmt"
	"strings"
	"time"

	"github.com/Rethunk-Tech/go-winsym"
)

// Source is the kind of record a Fault was read from.
type Source string

// The sources a Fault comes from.
const (
	SourceMinidump Source = "minidump"
	SourceUnityLog Source = "unity-error-log"
	SourceEventLog Source = "event-log"
	SourceCoredump Source = "coredump"
)

// Fault is the module a crash happened in. Fields a source does not carry stay zero.
type Fault struct {
	// Source is the kind of record the fault was read from, and Path the dump file or crash folder it came from, empty
	// for the event log and coredumpctl, which have none.
	Source Source
	Path   string
	// Time is when the record says the crash happened, or the file's modification time; zero when unknown.
	Time time.Time
	// Module is empty when the address lies in none of the process's modules.
	Module string
	// Code is the exception code, zero when the source names none or only names it in Detail.
	Code uint32
	// Detail is the exception in a few words, such as "exception 0xc0000005 at 0x7ff8a1b2c3d4".
	Detail string
	// Frame is, for an unhandled .NET exception, the first stack frame outside the runtime and the loader.
	Frame string
	// Managed is that exception's text and stack.
	Managed string
	// Offset is the crash address's offset into Module when HasOffset, and CodeView that module's debug record when
	// the dump carries it: what a symbol server needs to name the function. ModulePath is where the event log says
	// Module was loaded from, whose own record serves when the dump gave none.
	Offset     uint64
	HasOffset  bool
	CodeView   winsym.CodeView
	ModulePath string
}

// fromMinidump is a minidump's fault in the shape every crash source shares.
func fromMinidump(m winsym.Fault) Fault {
	f := Fault{Source: SourceMinidump, Code: m.Code, Detail: fmt.Sprintf("exception 0x%x at 0x%x", m.Code, m.Address), CodeView: m.CodeView}
	if m.Module != "" {
		f.Module, f.Offset, f.HasOffset = baseName(m.Module), m.Offset, true
	}
	return f
}

// baseName is the last element of a Windows or Unix path.
func baseName(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '\\' || p[i] == '/' {
			return p[i+1:]
		}
	}
	return p
}

// IsSystemModule reports whether name is a library every crash passes through on its way down (ntdll, kernel32, the C
// runtime, libc), never where one started.
func IsSystemModule(name string) bool {
	return systemModules[strings.ToLower(strings.TrimSuffix(name, ".dll"))] || systemModules[name]
}
