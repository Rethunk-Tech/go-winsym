package winsym

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
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
		if name, off, ok := r.Name(t.Context(), cv, 0x101c); !ok || name != "GfxDeviceD3D11::DrawBuffers" || off != 0x1c {
			t.Fatalf("%q +%#x %v", name, off, ok)
		}
	}
	other := cv
	other.Age = 2
	for range 2 {
		if _, _, ok := r.Name(t.Context(), other, 0x101c); ok {
			t.Fatal("a PDB the server lacks named a symbol")
		}
	}
	online = false
	other.Age = 3
	if _, _, ok := r.Name(t.Context(), other, 0x101c); ok || requests.Load() != 2 {
		t.Fatalf("requests = %d, want one for the PDB and one for the miss", requests.Load())
	}
}

// A 200 that is not a PDB is not cached, so the next URL can still supply the file.
func TestNameRejectsANonPDBBodyAndTriesTheNextURL(t *testing.T) {
	t.Parallel()
	cv := fixture(t)
	pdb := fixturePDBBytes(t)
	html := []byte("<html>denied</html>")
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(html)
	}))
	t.Cleanup(bad.Close)
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/"+fixturePDB+"/"+cv.Key()+"/"+fixturePDB {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(pdb)
	}))
	t.Cleanup(good.Close)
	r := &Resolver{Dir: t.TempDir(), Servers: []Server{{URL: bad.URL + "/"}, {URL: good.URL + "/"}}, Wait: 10 * time.Second}
	for range 2 {
		if name, _, ok := r.Name(t.Context(), cv, 0x101c); !ok || name != "GfxDeviceD3D11::DrawBuffers" {
			t.Fatalf("%q %v", name, ok)
		}
	}
	cached, err := os.ReadFile(filepath.Join(r.Dir, cv.PDB, cv.Key(), cv.PDB))
	if err != nil {
		t.Fatal(err)
	}
	if string(cached) != string(pdb) {
		t.Fatalf("cache held %d bytes, want the fixture PDB", len(cached))
	}
}

// A nil HTTP client downloads with http.DefaultClient instead of panicking in the download goroutine.
func TestResolverWithNoHTTPClientUsesTheDefault(t *testing.T) {
	t.Parallel()
	cv := fixture(t)
	var requests atomic.Int32
	srv := servePDB(t, cv, &requests)
	r := &Resolver{Dir: t.TempDir(), Servers: []Server{{URL: srv.URL + "/"}}, Wait: 10 * time.Second}
	if name, _, ok := r.Name(t.Context(), cv, 0x101c); !ok || name != "GfxDeviceD3D11::DrawBuffers" {
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
	if _, _, ok := r.Name(t.Context(), cv, 0x101c); ok || requests.Load() != 0 {
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

// A single fetched PDB larger than MaxCacheBytes is kept so Name can open it.
func TestNameKeepsAFetchedPDBOverTheCacheCap(t *testing.T) {
	t.Parallel()
	cv := fixture(t)
	var requests atomic.Int32
	srv := servePDB(t, cv, &requests)
	r := &Resolver{HTTP: srv.Client(), Dir: t.TempDir(), Servers: []Server{{URL: srv.URL + "/"}}, MaxCacheBytes: 1, Wait: 10 * time.Second}
	if name, _, ok := r.Name(t.Context(), cv, 0x101c); !ok || name != "GfxDeviceD3D11::DrawBuffers" {
		t.Fatalf("%q %v", name, ok)
	}
	if _, err := os.Stat(filepath.Join(r.Dir, cv.PDB, cv.Key(), cv.PDB)); err != nil {
		t.Fatalf("cached PDB missing: %v", err)
	}
}

// Downloads of different PDBs run at most maxDownloads at a time.
func TestDownloadsAreCapped(t *testing.T) {
	t.Parallel()
	var running, peak atomic.Int32
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := running.Add(1)
		defer running.Add(-1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		<-release
		http.NotFound(w, nil)
	}))
	t.Cleanup(srv.Close)
	r := &Resolver{HTTP: srv.Client(), Dir: t.TempDir(), Servers: []Server{{URL: srv.URL + "/"}}}
	var wg sync.WaitGroup
	for i := range 3 * maxDownloads {
		name := "m" + strconv.Itoa(i) + ".pdb"
		wg.Go(func() {
			<-r.fetch(t.Context(), filepath.Join(r.Dir, name, "k"), name, []string{srv.URL + "/" + name})
		})
	}
	deadline := time.Now().Add(5 * time.Second)
	for running.Load() < maxDownloads && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	// Give any download past the cap time to show itself.
	time.Sleep(200 * time.Millisecond)
	close(release)
	wg.Wait()
	if got := peak.Load(); got != maxDownloads {
		t.Fatalf("%d downloads ran at once, want %d", got, maxDownloads)
	}
}

func TestRedirectsNeverLeaveHTTPS(t *testing.T) {
	t.Parallel()
	get := func(url string) *http.Request {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
		if err != nil {
			t.Fatal(err)
		}
		return req
	}
	if err := refuseDowngrade(get("http://symbols.example/x"), []*http.Request{get("https://symbols.example/x")}); err == nil {
		t.Error("a redirect from https to http was followed")
	}
	if err := refuseDowngrade(get("https://other.example/x"), []*http.Request{get("https://symbols.example/x")}); err != nil {
		t.Errorf("an https redirect: %v", err)
	}
	if err := refuseDowngrade(get("http://b.example/x"), []*http.Request{get("http://a.example/x")}); err != nil {
		t.Errorf("an http redirect from http: %v", err)
	}
	if defaultClient.CheckRedirect == nil {
		t.Error("the default client follows every redirect")
	}
}

// A symlink in the cache pointing outside it is neither read nor touched.
func TestNameLeavesASymlinkTargetAlone(t *testing.T) {
	t.Parallel()
	cv := fixture(t)
	outside := filepath.Join(t.TempDir(), "outside.pdb")
	if err := os.WriteFile(outside, fixturePDBBytes(t), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	if err := os.Chtimes(outside, old, old); err != nil {
		t.Fatal(err)
	}
	r := &Resolver{Dir: t.TempDir(), Servers: []Server{{URL: "http://127.0.0.1:1/"}}, Online: func() bool { return false }}
	dir := filepath.Join(r.Dir, cv.PDB, cv.Key())
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, cv.PDB)); err != nil {
		t.Skip("no symlinks here:", err)
	}
	if _, _, ok := r.Name(t.Context(), cv, 0x101c); ok {
		t.Fatal("read through a symlink out of the cache")
	}
	if info, err := os.Stat(outside); err != nil || !info.ModTime().Equal(old) {
		t.Fatalf("the target was touched: %v %v", info.ModTime(), err)
	}
}
