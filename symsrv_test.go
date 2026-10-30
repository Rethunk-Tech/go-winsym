package winsym

import (
	"path/filepath"
	"strings"
	"testing"
)

const fixtureKey = "4FABE818577AB2304C4C44205044422E1"

func fixture(t *testing.T) CodeView {
	t.Helper()
	cv, ok := PECodeView(filepath.Join("testdata", "pe-codeview.dll"))
	if !ok || cv.PDB != fixturePDB || cv.Key() != fixtureKey {
		t.Fatalf("CodeView = %+v (key %s)", cv, cv.Key())
	}
	return cv
}

func TestPECodeViewReadsTheRecord(t *testing.T) {
	t.Parallel()
	fixture(t)
	for _, p := range []string{filepath.Join("testdata", "msf-publics.pdb"), filepath.Join("testdata", "absent.dll")} {
		if cv, ok := PECodeView(p); ok || cv != (CodeView{}) {
			t.Errorf("%s: %+v, %v", p, cv, ok)
		}
	}
}

func TestParseCodeViewDropsARejectedName(t *testing.T) {
	t.Parallel()
	for _, name := range []string{`..\x y.pdb`, `x.dll`, `..\..\`} {
		rec := append([]byte("RSDS0123456789abcdef\x01\x00\x00\x00"), name+"\x00"...)
		if cv, ok := parseCodeView(rec); ok || cv != (CodeView{}) {
			t.Errorf("%q: %+v, %v", name, cv, ok)
		}
	}
	rec := []byte("RSDS0123456789abcdef\x01\x00\x00\x00..\\a/b\\ok.pdb\x00")
	if cv, ok := parseCodeView(rec); !ok || cv.PDB != "ok.pdb" || cv.Age != 1 {
		t.Errorf("accepted record: %+v, %v", cv, ok)
	}
}

func TestDefaultServersRouteByPDBName(t *testing.T) {
	t.Parallel()
	cv := fixture(t)
	unity := urls(DefaultServers(), cv)
	if len(unity) != 2 || unity[0] != "https://symbolserver.unity3d.com/"+fixturePDB+"/"+fixtureKey+"/"+fixturePDB || !strings.HasPrefix(unity[1], "http://") {
		t.Errorf("Unity's PDB: %v", unity)
	}
	for name, server := range map[string]string{"NTDLL.pdb": "https://msdl.microsoft.com/", "vcruntime140_1.amd64.pdb": "https://msdl.microsoft.com/", "Cool.pdb": "", "game.pdb": "", "../ntdll.pdb": ""} {
		got := urls(DefaultServers(), CodeView{PDB: name, GUID: cv.GUID, Age: cv.Age})
		if server == "" && len(got) != 0 || server != "" && (len(got) != 1 || !strings.HasPrefix(got[0], server)) {
			t.Errorf("%s: %v", name, got)
		}
	}
}

func TestServerWithNoServesIsAskedForEveryName(t *testing.T) {
	t.Parallel()
	servers := []Server{{URL: "https://a/"}, {URL: "https://b/", Serves: func(string) bool { return false }}}
	got := urls(servers, CodeView{PDB: "Any.pdb", Age: 1})
	if len(got) != 1 || !strings.HasPrefix(got[0], "https://a/Any.pdb/") {
		t.Errorf("urls = %v", got)
	}
}

func TestKeyOrdersTheGUIDFieldsLikeWindows(t *testing.T) {
	t.Parallel()
	cv := CodeView{GUID: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Age: 0x1a}
	if got, want := cv.Key(), "04030201"+"0605"+"0807"+"090A0B0C0D0E0F10"+"1A"; got != want {
		t.Errorf("Key = %s, want %s", got, want)
	}
}
