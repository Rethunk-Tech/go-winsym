package winsym

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
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
		_, _, _ = Lookup(bytes.NewReader(data), int64(len(data)), rva)
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
