package winsym

import (
	"bytes"
	"encoding/binary"
	"math"
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

func TestLookupRejectsAStructurallyUnsoundPDB(t *testing.T) {
	t.Parallel()
	le := binary.LittleEndian
	header := func(block, nblocks, dirBytes, mapIdx uint32, file int) []byte {
		b := make([]byte, file)
		copy(b, msfMagic)
		le.PutUint32(b[32:], block)
		le.PutUint32(b[40:], nblocks)
		le.PutUint32(b[44:], dirBytes)
		le.PutUint32(b[52:], mapIdx)
		return b
	}
	listOverflow := header(512, 3, 8, 2, 1536)
	le.PutUint32(listOverflow[2*512:], 1)
	le.PutUint32(listOverflow[512:], 1)
	le.PutUint32(listOverflow[516:], 512)
	for _, c := range []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"unsupported block size", header(256, 1, 4, 0, 256)},
		{"file shorter than its block count", header(512, 2, 4, 0, 56)},
		{"directory shorter than a stream count", header(512, 1, 0, 0, 512)},
		{"directory longer than the file", header(512, 1, 400, 0, 512)},
		{"block map past the last block", header(512, 1, 4, 1, 512)},
		{"directory map wider than one block", header(512, 200, 129*512, 1, 200*512)},
		{"stream count past the directory", header(512, 2, 8, 1, 1024)},
		{"block list past the directory", listOverflow},
		{"too few streams", msfPDB(t, [][]byte{{}, {}, {}})},
		{"DBI shorter than its header", msfPDB(t, [][]byte{{}, {}, {}, make([]byte, 10)})},
		{"DBI without a debug header", msfPDB(t, [][]byte{{}, {}, {}, make([]byte, 64)})},
		{"section header stream missing", msfPDB(t, [][]byte{{}, {}, {}, dbi(4, 9), {}})},
		{"public stream missing", msfPDB(t, [][]byte{{}, {}, {}, dbi(9, 4), sec(0x1000)})},
	} {
		if _, _, err := Lookup(bytes.NewReader(c.data), int64(len(c.data)), 0x1000); err == nil {
			t.Errorf("%s: named a function", c.name)
		}
	}
}

func TestLookupNamesAFunctionPastSkippedPublicRecords(t *testing.T) {
	t.Parallel()
	var recs []byte
	recs = append(recs, pubRecord(0x0001, pubFunction, 0, 1, "SkipKind")...)
	recs = append(recs, pubRecord(sPub32, 0, 0, 1, "SkipData")...)
	recs = append(recs, pubRecord(sPub32, pubFunction, 0, 99, "SkipSeg")...)
	recs = append(recs, pubRecord(sPub32, pubFunction, 0x10, 1, "WinMain")...)
	recs = append(recs, 50, 0, 0, 0)
	data := msfPDB(t, [][]byte{nil, {}, {}, dbi(4, 5), recs, sec(0x1000)})
	name, off, err := Lookup(bytes.NewReader(data), int64(len(data)), 0x101c)
	if err != nil || name != "WinMain" || off != 0xc {
		t.Fatalf("%q +%#x %v", name, off, err)
	}
}

func u16(n int) uint16 {
	if n < 0 || n > math.MaxUint16 {
		panic("fixture offset out of range")
	}
	return uint16(n)
}

func dbi(pub, headers uint16) []byte {
	b := make([]byte, 76)
	binary.LittleEndian.PutUint16(b[20:], pub)
	binary.LittleEndian.PutUint16(b[74:], headers)
	return b
}

func sec(va uint32) []byte {
	h := make([]byte, 40)
	binary.LittleEndian.PutUint32(h[12:], va)
	return h
}

func pubRecord(kind uint16, flags, off uint32, seg uint16, name string) []byte {
	rec := make([]byte, 12+len(name)+1)
	le := binary.LittleEndian
	le.PutUint16(rec, kind)
	le.PutUint32(rec[2:], flags)
	le.PutUint32(rec[6:], off)
	le.PutUint16(rec[10:], seg)
	copy(rec[12:], name)
	out := make([]byte, 2+len(rec))
	le.PutUint16(out, u16(len(rec)))
	copy(out[2:], rec)
	return out
}

// A nil stream is MSF's deleted-stream size, 0xFFFFFFFF, not an empty stream.
func msfPDB(t *testing.T, streams [][]byte) []byte {
	t.Helper()
	const block = 512
	le := binary.LittleEndian
	chunks := map[uint32][]byte{}
	var next uint32 = 1
	store := func(data []byte) []uint32 {
		if len(data) == 0 {
			return nil
		}
		var idx []uint32
		for off := 0; off < len(data); off += block {
			end := min(off+block, len(data))
			chunks[next] = append([]byte(nil), data[off:end]...)
			idx = append(idx, next)
			next++
		}
		return idx
	}
	type info struct {
		size   uint32
		blocks []uint32
	}
	infos := make([]info, len(streams))
	for i, s := range streams {
		if s == nil {
			infos[i].size = 0xFFFFFFFF
			continue
		}
		infos[i].size = u32(len(s))
		infos[i].blocks = store(s)
	}
	var dir []byte
	dir = le.AppendUint32(dir, u32(len(streams)))
	for _, inf := range infos {
		dir = le.AppendUint32(dir, inf.size)
	}
	for _, inf := range infos {
		for _, b := range inf.blocks {
			dir = le.AppendUint32(dir, b)
		}
	}
	dirBlocks := store(dir)
	var mapBytes []byte
	for _, b := range dirBlocks {
		mapBytes = le.AppendUint32(mapBytes, b)
	}
	mapIdx := next
	chunks[next] = mapBytes
	next++
	out := make([]byte, int(next)*block)
	copy(out, msfMagic)
	le.PutUint32(out[32:], block)
	le.PutUint32(out[40:], next)
	le.PutUint32(out[44:], u32(len(dir)))
	le.PutUint32(out[52:], mapIdx)
	for i, c := range chunks {
		copy(out[int(i)*block:], c)
	}
	return out
}
