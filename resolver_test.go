package winsym

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func fixturePDBBytes(t *testing.T) []byte {
	t.Helper()
	pdb, err := os.ReadFile(filepath.Join("testdata", "msf-publics.pdb"))
	if err != nil {
		t.Fatal(err)
	}
	return pdb
}

// servePDB answers only the fixture's symbol-store path and counts every request.
func servePDB(t *testing.T, cv CodeView, requests *atomic.Int32) *httptest.Server {
	t.Helper()
	pdb := fixturePDBBytes(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/"+fixturePDB+"/"+cv.Key()+"/"+fixturePDB {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(pdb)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// A PDB is fetched once and then read from the cache, a 404 is remembered, and nothing is fetched while offline.
func TestResolverFetchesOnceAndCaches(t *testing.T) {
	t.Parallel()
	cv := fixture(t)
	var requests atomic.Int32
	srv := servePDB(t, cv, &requests)
	online := true
	r := &Resolver{HTTP: srv.Client(), Dir: t.TempDir(), Servers: []Server{{URL: srv.URL + "/"}}, Online: func() bool { return online }, Wait: 10 * time.Second}
	dir := filepath.Join(r.Dir, cv.PDB, cv.Key())
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, cv.PDB+".part"), []byte("left by a crash"), 0o600); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if name, off, ok := r.Name(context.Background(), cv, 0x101c); !ok || name != "GfxDeviceD3D11::DrawBuffers" || off != 0x1c {
			t.Fatalf("%q +%#x %v", name, off, ok)
		}
	}
	other := cv
	other.Age = 2
	for range 2 {
		if _, _, ok := r.Name(context.Background(), other, 0x101c); ok {
			t.Fatal("a PDB the server lacks named a symbol")
		}
	}
	online = false
	other.Age = 3
	if _, _, ok := r.Name(context.Background(), other, 0x101c); ok || requests.Load() != 2 {
		t.Fatalf("requests = %d, want one for the PDB and one for the miss", requests.Load())
	}
}

// A nil HTTP client downloads with http.DefaultClient instead of panicking in the download goroutine.
func TestResolverWithNoHTTPClientUsesTheDefault(t *testing.T) {
	t.Parallel()
	cv := fixture(t)
	var requests atomic.Int32
	srv := servePDB(t, cv, &requests)
	r := &Resolver{Dir: t.TempDir(), Servers: []Server{{URL: srv.URL + "/"}}, Wait: 10 * time.Second}
	if name, _, ok := r.Name(context.Background(), cv, 0x101c); !ok || name != "GfxDeviceD3D11::DrawBuffers" {
		t.Fatalf("%q %v", name, ok)
	}
}

// A PDB no listed server publishes is not requested and not read from the cache.
func TestResolverIgnoresAPDBNoServerServes(t *testing.T) {
	t.Parallel()
	cv := fixture(t)
	var requests atomic.Int32
	srv := servePDB(t, cv, &requests)
	r := &Resolver{HTTP: srv.Client(), Dir: t.TempDir(), Servers: []Server{{URL: srv.URL + "/", Serves: func(string) bool { return false }}}, Wait: time.Second}
	dir := filepath.Join(r.Dir, cv.PDB, cv.Key())
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, cv.PDB), fixturePDBBytes(t), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := r.Name(context.Background(), cv, 0x101c); ok || requests.Load() != 0 {
		t.Fatalf("ok = %v after %d requests", ok, requests.Load())
	}
}

// A miss marker is retried after missMaxAge, and prune clears expired markers and the folders they leave empty.
func TestMissExpires(t *testing.T) {
	t.Parallel()
	r := &Resolver{Dir: t.TempDir()}
	old := time.Now().Add(-missMaxAge - time.Hour)
	stale := filepath.Join(r.Dir, "a.pdb", "KEY1")
	fresh := filepath.Join(r.Dir, "b.pdb", "KEY2")
	for _, d := range []string{stale, fresh} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, missMarker), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chtimes(filepath.Join(stale, missMarker), old, old); err != nil {
		t.Fatal(err)
	}
	if freshMiss(stale) || !freshMiss(fresh) {
		t.Fatalf("freshMiss: stale %v, fresh %v", freshMiss(stale), freshMiss(fresh))
	}
	r.prune()
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}
	r.prune()
	if _, err := os.Stat(filepath.Join(r.Dir, "a.pdb")); !os.IsNotExist(err) {
		t.Fatalf("stale miss folders remain: %v", err)
	}
	if !freshMiss(fresh) {
		t.Fatal("a fresh miss was pruned")
	}
}

func TestPruneKeepsStrayFiles(t *testing.T) {
	t.Parallel()
	r := &Resolver{Dir: t.TempDir()}
	note := filepath.Join(r.Dir, "notes.txt")
	if err := os.WriteFile(note, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	r.prune()
	if _, err := os.Stat(note); err != nil {
		t.Fatalf("prune removed a file it does not own: %v", err)
	}
}

// The least recently modified PDB goes first when the cache is over MaxCacheBytes.
func TestPruneEvictsTheOldestPDBOverTheCap(t *testing.T) {
	t.Parallel()
	r := &Resolver{Dir: t.TempDir(), MaxCacheBytes: 10}
	older, newer := filepath.Join(r.Dir, "a.pdb"), filepath.Join(r.Dir, "b.pdb")
	for _, p := range []string{older, newer} {
		if err := os.WriteFile(p, make([]byte, 8), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(older, past, past); err != nil {
		t.Fatal(err)
	}
	r.prune()
	if _, err := os.Stat(older); !os.IsNotExist(err) {
		t.Errorf("older PDB remains: %v", err)
	}
	if _, err := os.Stat(newer); err != nil {
		t.Errorf("newer PDB removed: %v", err)
	}
}
