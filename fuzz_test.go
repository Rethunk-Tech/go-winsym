package winsym

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func FuzzLookup(f *testing.F) {
	pdb, err := os.ReadFile(filepath.Join("testdata", "msf-publics.pdb"))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(pdb, uint64(0x101c))
	f.Add(pdb, uint64(0x1050))
	f.Add(pdb, uint64(0x10))
	f.Add([]byte{}, uint64(0))
	f.Add([]byte(msfMagic), uint64(0))
	f.Fuzz(func(t *testing.T, data []byte, rva uint64) {
		name, offset, err := Lookup(bytes.NewReader(data), int64(len(data)), rva)
		if err != nil {
			if name != "" || offset != 0 {
				t.Fatalf("error %v with name %q offset %#x", err, name, offset)
			}
			return
		}
		if name == "" {
			t.Fatal("named an empty function")
		}
		if offset > rva {
			t.Fatalf("offset %#x is past RVA %#x", offset, rva)
		}
	})
}

func FuzzParseCodeView(f *testing.F) {
	f.Add([]byte("RSDS0123456789abcdef\x01\x00\x00\x00C:\\build\\Cool.pdb\x00"))
	f.Add([]byte("RSDS0123456789abcdef\x01\x00\x00\x00..\\a/b\\ok.pdb\x00"))
	f.Add([]byte("RSDS0123456789abcdef\x01\x00\x00\x00FOO.PDB\x00"))
	f.Add([]byte("RSDS0123456789abcdef\x01\x00\x00\x00..\\x y.pdb\x00"))
	f.Add([]byte("RSDS0123456789abcdef\x01\x00\x00\x00UnityPlayer_Win64_player_mono_x64.pdb\x00"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		cv, ok := parseCodeView(data)
		if !ok {
			if cv != (CodeView{}) {
				t.Fatalf("failed parse returned %+v", cv)
			}
			return
		}
		if !pdbName.MatchString(cv.PDB) || strings.ContainsAny(cv.PDB, `/\`) {
			t.Fatalf("PDB name %q would leave the cache folder", cv.PDB)
		}
	})
}

func FuzzPECodeView(f *testing.F) {
	dll, err := os.ReadFile(filepath.Join("testdata", "pe-codeview.dll"))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(dll)
	f.Add([]byte{})
	f.Add([]byte("MZ"))
	f.Fuzz(func(t *testing.T, data []byte) {
		p := filepath.Join(t.TempDir(), "x")
		if err := os.WriteFile(p, data, 0o600); err != nil {
			t.Fatal(err)
		}
		cv, ok := PECodeView(p)
		if ok && !pdbName.MatchString(cv.PDB) {
			t.Fatalf("PDB name %q would leave the cache folder", cv.PDB)
		}
		if !ok && cv != (CodeView{}) {
			t.Fatalf("failed parse returned %+v", cv)
		}
	})
}

func FuzzUndecorate(f *testing.F) {
	f.Add("??0Gfx@Unity@@QEAA@XZ")
	f.Add("??1Gfx@@QEAA@XZ")
	f.Add("?DrawBuffers@GfxDeviceD3D11@@QEAAHPEAH@Z")
	f.Add("??_GFoo@@UEAAPEAXI@Z")
	f.Add("plain_c")
	f.Add("?")
	f.Fuzz(func(t *testing.T, s string) {
		got := undecorate(s)
		if !strings.ContainsRune(s, 0) && strings.ContainsRune(got, 0) {
			t.Fatalf("undecorate inserted a NUL: %q -> %q", s, got)
		}
		if !strings.HasPrefix(s, "?") && got != s {
			t.Fatalf("plain name mutated: %q -> %q", s, got)
		}
		if utf8.ValidString(s) && !utf8.ValidString(got) {
			t.Fatalf("undecorate broke UTF-8: %q -> %q", s, got)
		}
	})
}
