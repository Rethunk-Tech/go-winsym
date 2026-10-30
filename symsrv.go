package winsym

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
)

const (
	codeViewMagic  = 0x53445352 // "RSDS"
	debugCodeView  = 2
	debugEntrySize = 28
	maxCodeView    = 1 << 10
)

// pdbName is a PDB file name a server path and the cache can hold: no folder, no traversal.
var pdbName = regexp.MustCompile(`^[A-Za-z0-9_.-]+\.pdb$`)

// CodeView is a module's RSDS debug record: the PDB it was linked with and that PDB's identity.
type CodeView struct {
	// PDB is a base name only and always matches ^[A-Za-z0-9_.-]+\.pdb$ when non-empty.
	PDB  string
	GUID [16]byte
	Age  uint32
}

// parseCodeView reads an RSDS record: the magic, the GUID, the age and the PDB path. Only the path's base name is
// kept, and a record whose name fails pdbName yields the zero CodeView.
func parseCodeView(b []byte) (CodeView, bool) {
	if len(b) < 25 || binary.LittleEndian.Uint32(b) != codeViewMagic {
		return CodeView{}, false
	}
	var cv CodeView
	copy(cv.GUID[:], b[4:20])
	cv.Age = binary.LittleEndian.Uint32(b[20:])
	path, _, _ := bytes.Cut(b[24:], []byte{0})
	name := string(path)
	cv.PDB = name[strings.LastIndexAny(name, `/\`)+1:]
	if !pdbName.MatchString(cv.PDB) {
		return CodeView{}, false
	}
	return cv, true
}

// PECodeView reads the CodeView record of the PE file at path. The path is the caller's to vet: the file is opened
// as given and only its debug directory is read.
func PECodeView(path string) (CodeView, bool) {
	file, err := os.Open(path) //nolint:gosec // G304: reading the caller's named file is the function's purpose
	if err != nil {
		return CodeView{}, false
	}
	defer func() { _ = file.Close() }()
	f, err := pe.NewFile(file)
	if err != nil {
		return CodeView{}, false
	}
	var dir pe.DataDirectory
	switch h := f.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		dir = h.DataDirectory[pe.IMAGE_DIRECTORY_ENTRY_DEBUG]
	case *pe.OptionalHeader64:
		dir = h.DataDirectory[pe.IMAGE_DIRECTORY_ENTRY_DEBUG]
	}
	if dir.Size == 0 || dir.Size > 64*debugEntrySize {
		return CodeView{}, false
	}
	for _, s := range f.Sections {
		if dir.VirtualAddress < s.VirtualAddress || dir.VirtualAddress+dir.Size > s.VirtualAddress+s.Size {
			continue
		}
		entries := make([]byte, dir.Size)
		if _, err := s.ReadAt(entries, int64(dir.VirtualAddress-s.VirtualAddress)); err != nil {
			return CodeView{}, false
		}
		for e := entries; len(e) >= debugEntrySize; e = e[debugEntrySize:] {
			if binary.LittleEndian.Uint32(e[12:]) != debugCodeView {
				continue
			}
			raw := make([]byte, min(binary.LittleEndian.Uint32(e[16:]), maxCodeView))
			if _, err := file.ReadAt(raw, int64(binary.LittleEndian.Uint32(e[24:]))); err != nil {
				return CodeView{}, false
			}
			return parseCodeView(raw)
		}
	}
	return CodeView{}, false
}

// Key is the folder a symbol server files the PDB under: the GUID as 32 upper-case hex digits, its first three fields
// little-endian as Windows stores them, then the age in hex.
func (cv CodeView) Key() string {
	g := cv.GUID
	return fmt.Sprintf("%08X%04X%04X%X%X", binary.LittleEndian.Uint32(g[0:]), binary.LittleEndian.Uint16(g[4:]),
		binary.LittleEndian.Uint16(g[6:]), g[8:16], cv.Age)
}

// Server is a symbol server and the PDB names it is asked for.
type Server struct {
	// URL is the base, ending in "/"; "<pdb>/<key>/<pdb>" is appended.
	URL string
	// Serves gets the lower-cased PDB name and says whether this server publishes it. Nil asks for every name.
	Serves func(pdb string) bool
}

var (
	unityPrefixes = []string{"unityplayer_", "windowsplayer_", "baselib_"}
	// microsoftNames are the system and runtime PDBs a crash in a Windows process passes through.
	microsoftNames = []string{
		"ntdll.pdb", "kernelbase.pdb", "kernel32.pdb", "ucrtbase.pdb", "vcruntime140.amd64.pdb",
		"vcruntime140_1.amd64.pdb", "msvcp140.amd64.pdb", "user32.pdb", "win32u.pdb", "d3d11.pdb", "dxgi.pdb",
		"coreclr.pdb", "clrjit.pdb",
	}
)

func servesUnity(pdb string) bool {
	return slices.ContainsFunc(unityPrefixes, func(p string) bool { return strings.HasPrefix(pdb, p) })
}

func servesMicrosoft(pdb string) bool { return slices.Contains(microsoftNames, pdb) }

// DefaultServers is Unity's symbol server (https, then http: Unity documents only the http address) for
// unityplayer_, windowsplayer_ and baselib_ PDBs, and Microsoft's for the system and runtime PDBs a crash in a Windows
// process passes through. A PDB no listed server publishes is never requested, so a private module's name does
// not reach a third party. Each call returns a fresh slice.
func DefaultServers() []Server {
	return []Server{
		{URL: "https://symbolserver.unity3d.com/", Serves: servesUnity},
		{URL: "http://symbolserver.unity3d.com/", Serves: servesUnity},
		{URL: "https://msdl.microsoft.com/download/symbols/", Serves: servesMicrosoft},
	}
}

// urls are where the servers that publish the PDB serve it, in list order; none for a PDB name that fails pdbName.
func urls(servers []Server, cv CodeView) []string {
	if !pdbName.MatchString(cv.PDB) {
		return nil
	}
	name := strings.ToLower(cv.PDB)
	path := cv.PDB + "/" + cv.Key() + "/" + cv.PDB
	var out []string
	for _, s := range servers {
		if s.Serves == nil || s.Serves(name) {
			out = append(out, s.URL+path)
		}
	}
	return out
}
