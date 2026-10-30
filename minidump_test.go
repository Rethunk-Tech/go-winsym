package winsym

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
	"unicode/utf16"
)

type testModule struct {
	name string
	base uint64
	size uint32
	cv   []byte
}

// u32 narrows a test fixture's offset or count, which always fits.
func u32(n int) uint32 {
	if n < 0 || n > math.MaxUint32 {
		panic("fixture offset out of range")
	}
	return uint32(n)
}

// minidump builds the header, directory, exception stream and module list of a minidump.
func minidump(addr uint64, mods ...testModule) []byte {
	le := binary.LittleEndian
	const dirRva, excRva, modRva = 32, 32 + 2*12, 32 + 2*12 + 168
	var buf bytes.Buffer
	put := func(v any) { _ = binary.Write(&buf, le, v) }
	put(uint32(minidumpMagic))
	put(uint32(0xa793))
	put(uint32(2))
	put(uint32(dirRva))
	put(make([]byte, 16))
	put([]uint32{exceptionStream, 168, excRva})
	put([]uint32{moduleListStream, u32(4 + len(mods)*moduleEntrySize), modRva})
	exc := make([]byte, 168)
	le.PutUint32(exc[8:], 0xc0000005)
	le.PutUint64(exc[24:], addr)
	buf.Write(exc)
	put(u32(len(mods)))
	strAt := modRva + 4 + len(mods)*moduleEntrySize
	var strs bytes.Buffer
	for _, m := range mods {
		entry := make([]byte, moduleEntrySize)
		le.PutUint64(entry[0:], m.base)
		le.PutUint32(entry[8:], m.size)
		le.PutUint32(entry[20:], u32(strAt+strs.Len()))
		u := utf16.Encode([]rune(m.name))
		_ = binary.Write(&strs, le, u32(len(u)*2))
		_ = binary.Write(&strs, le, u)
		if m.cv != nil {
			le.PutUint32(entry[76:], u32(len(m.cv)))
			le.PutUint32(entry[80:], u32(strAt+strs.Len()))
			strs.Write(m.cv)
		}
		buf.Write(entry)
	}
	buf.Write(strs.Bytes())
	return buf.Bytes()
}

func TestMinidumpNamesTheModuleHoldingTheFaultAddress(t *testing.T) {
	t.Parallel()
	dump := minidump(0x7ff800001234,
		testModule{`C:\Game\Game.exe`, 0x140000000, 0x2000000, nil},
		testModule{`C:\Game\Mods\Cool\Cool.dll`, 0x7ff800000000, 0x10000, []byte("RSDS0123456789abcdef\x02\x00\x00\x00C:\\build\\Cool.pdb\x00")},
		testModule{`C:\Windows\System32\ntdll.dll`, 0x7ff900000000, 0x200000, nil},
	)
	got, err := ParseMinidump(bytes.NewReader(dump), int64(len(dump)))
	if err != nil || got.Code != 0xc0000005 || got.Address != 0x7ff800001234 || got.Module != `C:\Game\Mods\Cool\Cool.dll` || got.Offset != 0x1234 || got.CodeView.PDB != "Cool.pdb" || got.CodeView.Age != 2 {
		t.Fatalf("fault = %+v, %v", got, err)
	}
}

func TestMinidumpWithAnAddressInNoModuleNamesNone(t *testing.T) {
	t.Parallel()
	dump := minidump(0x1, testModule{`C:\Game\Game.exe`, 0x140000000, 0x2000000, nil})
	got, err := ParseMinidump(bytes.NewReader(dump), int64(len(dump)))
	if err != nil || got.Module != "" || got.Address != 1 {
		t.Fatalf("fault = %+v, %v", got, err)
	}
}

func TestMinidumpRejectsGarbageAndTruncation(t *testing.T) {
	t.Parallel()
	for name, data := range map[string][]byte{
		"empty":     nil,
		"not a dmp": bytes.Repeat([]byte("x"), 200),
		"truncated": minidump(0x1, testModule{"a.dll", 0, 10, nil})[:60],
	} {
		if _, err := ParseMinidump(bytes.NewReader(data), int64(len(data))); err == nil {
			t.Errorf("%s: parsed", name)
		}
	}
}

// FuzzMinidump feeds a dump as any program, or a crash it did not cause, could have written it.
func FuzzMinidump(f *testing.F) {
	valid := minidump(0x7ff800001234,
		testModule{`C:\Game\Cool.dll`, 0x7ff800000000, 0x10000, []byte("RSDS0123456789abcdef\x02\x00\x00\x00C:\\build\\Cool.pdb\x00")})
	f.Add(valid)
	f.Add(valid[:len(valid)/2])
	f.Add(minidump(0x7ff800001234,
		testModule{`C:\Game\Cool.dll`, 0x7ff800000000, 0x10000, []byte("RSDS0123456789abcdef\x02\x00\x00\x00..\\x.pdb\x00")}))
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := ParseMinidump(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return
		}
		if got.CodeView.PDB != "" && !pdbName.MatchString(got.CodeView.PDB) {
			t.Fatalf("PDB name %q would leave the cache folder", got.CodeView.PDB)
		}
	})
}
