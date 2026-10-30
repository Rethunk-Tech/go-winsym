package winsym

import (
	"os"
	"path/filepath"
	"testing"
)

// testdata/pe-codeview.dll and testdata/msf-publics.pdb are built by testdata/src/build.sh: DrawBuffers at RVA 0x1000,
// UnityMain at 0x1030. The PDB's GUID is {4FABE818-577A-B230-4C4C-44205044422E}, age 1, which is the server key once
// its first three fields are read little-endian.
const fixturePDB = "UnityPlayer_Win64_player_mono_x64.pdb"

func TestLookupNamesTheNearestPublicFunction(t *testing.T) {
	t.Parallel()
	f, err := os.Open(filepath.Join("testdata", "msf-publics.pdb"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		rva    uint64
		name   string
		offset uint64
	}{{0x101c, "GfxDeviceD3D11::DrawBuffers", 0x1c}, {0x1050, "UnityMain", 0x20}, {0x10, "", 0}} {
		name, offset, err := Lookup(f, info.Size(), c.rva)
		if name != c.name || offset != c.offset || (err == nil) != (c.name != "") {
			t.Errorf("%#x: %q +%#x, %v", c.rva, name, offset, err)
		}
	}
}

func TestUndecorate(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"??0Gfx@Unity@@QEAA@XZ": "Unity::Gfx::Gfx",
		"??1Gfx@@QEAA@XZ":       "Gfx::~Gfx",
		"??_GFoo@@UEAAPEAXI@Z":  "??_GFoo@@UEAAPEAXI@Z",
		"plain_c":               "plain_c",
	} {
		if got := undecorate(in); got != want {
			t.Errorf("undecorate(%s) = %s, want %s", in, got, want)
		}
	}
}
