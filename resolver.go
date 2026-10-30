package winsym

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	// maxPDBBytes bounds one download; a player's PDB is tens of megabytes.
	maxPDBBytes = 512 << 20
	// defaultCacheBytes bounds the cache folder when Resolver.MaxCacheBytes is zero.
	defaultCacheBytes = 2 << 30
	missMarker        = "miss"
	// partMaxAge is how long a .part may sit before prune treats it as abandoned (a download is capped at 10 minutes).
	partMaxAge = time.Hour
	// missMaxAge is how long a miss marker stands before the server is asked again (a symbol server may publish later).
	missMaxAge = 7 * 24 * time.Hour
)

var errMissing = errors.New("the symbol server has no such PDB")

// Resolver fetches PDBs into Dir in the symbol-store layout <Dir>/<pdb>/<key>/<pdb> and reads them. Name reads the
// cache only for a PDB some Server serves: the name is validated and the server list consulted before any path is
// built, so a PDB placed in Dir by hand for a module no Server serves is never read; use Lookup, or add a Server, for
// those.
type Resolver struct {
	// Dir must belong to this Resolver alone: prune deletes every *.pdb under it, least recently modified first,
	// and every *.pdb.part older than an hour, whoever wrote them.
	Dir string
	// HTTP is the client for downloads; nil means http.DefaultClient. Each download also carries a 10-minute context.
	HTTP *http.Client
	// Servers are asked in order; nil means DefaultServers().
	Servers []Server
	// Online says whether to try the network; nil means always. A cached PDB is read either way.
	Online func() bool
	// Wait is how long Name waits for a download before answering without it; the download carries on, so the next
	// call has the name.
	Wait time.Duration
	// MaxCacheBytes caps the cache folder; zero means 2 GiB. The least recently read PDBs go first.
	MaxCacheBytes int64

	mu       sync.Mutex
	inflight map[string]chan struct{}
}

// Name is the public symbol the address rva of the module cv identifies lies in and the offset into it, such as
// "GfxDeviceD3D11::DrawBuffers". ok is false when no server publishes its PDB, the PDB is not cached and cannot be
// fetched now, or no symbol covers rva.
func (r *Resolver) Name(ctx context.Context, cv CodeView, rva uint64) (name string, offset uint64, ok bool) {
	servers := r.Servers
	if servers == nil {
		servers = DefaultServers()
	}
	urls := urls(servers, cv)
	if len(urls) == 0 || r.Dir == "" {
		return "", 0, false
	}
	dir := filepath.Join(r.Dir, cv.PDB, cv.Key())
	path := filepath.Join(dir, cv.PDB)
	if !isFile(path) {
		if freshMiss(dir) || r.Online != nil && !r.Online() {
			return "", 0, false
		}
		select {
		case <-r.fetch(context.WithoutCancel(ctx), dir, cv.PDB, urls):
		case <-time.After(r.Wait):
			return "", 0, false
		case <-ctx.Done():
			return "", 0, false
		}
	}
	f, err := os.OpenInRoot(dir, cv.PDB)
	if err != nil {
		return "", 0, false
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return "", 0, false
	}
	now := time.Now()
	_ = os.Chtimes(path, now, now)
	sym, off, err := Lookup(f, info.Size(), rva)
	return sym, off, err == nil
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// fetch downloads the PDB once however many callers wait for it, and closes the returned channel when done. ctx
// should not end with the caller who gave up waiting: the download carries on for the next one.
func (r *Resolver) fetch(ctx context.Context, dir, pdb string, urls []string) chan struct{} {
	path := filepath.Join(dir, pdb)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.inflight == nil {
		r.inflight = map[string]chan struct{}{}
	}
	if done, ok := r.inflight[path]; ok {
		return done
	}
	done := make(chan struct{})
	r.inflight[path] = done
	go func() {
		defer func() {
			r.mu.Lock()
			delete(r.inflight, path)
			r.mu.Unlock()
			close(done)
		}()
		r.download(ctx, dir, pdb, urls)
	}()
	return done
}

func freshMiss(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, missMarker))
	return err == nil && time.Since(info.ModTime()) <= missMaxAge
}

func (r *Resolver) download(ctx context.Context, dir, pdb string, urls []string) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return
	}
	// A root confines every write to the key folder, so the names below cannot leave it.
	root, err := os.OpenRoot(dir)
	if err != nil {
		return
	}
	defer func() { _ = root.Close() }()
	missing := 0
	for _, u := range urls {
		err := r.get(ctx, root, u, pdb)
		if err == nil {
			r.prune()
			return
		}
		if errors.Is(err, errMissing) {
			missing++
		}
	}
	if missing == len(urls) {
		_ = root.WriteFile(missMarker, nil, 0o600)
	}
}

func (r *Resolver) get(ctx context.Context, root *os.Root, url, pdb string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	client := r.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return errMissing
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("%s: %s", url, resp.Status)
	case resp.ContentLength > maxPDBBytes:
		return fmt.Errorf("%s: %d bytes is over the cap", url, resp.ContentLength)
	}
	part := pdb + ".part"
	// Only the in-flight download of this path gets here, so a .part already present is a crash's leftover.
	_ = retry(func() error { return root.Remove(part) })
	tmp, err := root.OpenFile(part, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	// One byte past the cap tells a body that is exactly the cap from one that exceeds it.
	n, err := io.Copy(tmp, io.LimitReader(resp.Body, maxPDBBytes+1))
	if err == nil && n > maxPDBBytes {
		err = fmt.Errorf("%s: over the %d-byte cap", url, maxPDBBytes)
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = retry(func() error { return root.Remove(part) })
		return err
	}
	return retry(func() error { return root.Rename(part, pdb) })
}

// prune deletes the least recently read PDBs until the cache is under its cap.
func (r *Resolver) prune() {
	type entry struct {
		path string
		size int64
		used time.Time
	}
	var all []entry
	var total int64
	_ = filepath.WalkDir(r.Dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if strings.HasSuffix(p, ".pdb.part") {
			if info, err := d.Info(); err == nil && time.Since(info.ModTime()) > partMaxAge {
				_ = os.Remove(p)
			}
			return nil
		}
		if !strings.HasSuffix(p, ".pdb") {
			return nil
		}
		if info, err := d.Info(); err == nil {
			all = append(all, entry{p, info.Size(), info.ModTime()})
			total += info.Size()
		}
		return nil
	})
	r.pruneMisses()
	slices.SortFunc(all, func(a, b entry) int { return a.used.Compare(b.used) })
	limit := r.MaxCacheBytes
	if limit == 0 {
		limit = defaultCacheBytes
	}
	for _, e := range all {
		if total <= limit {
			return
		}
		if os.Remove(e.path) == nil {
			total -= e.size
		}
	}
}

// pruneMisses removes expired miss markers, then the key and PDB folders left empty. An empty folder younger than
// partMaxAge may belong to a download that has not written yet, so it stays.
func (r *Resolver) pruneMisses() {
	pdbs, _ := os.ReadDir(r.Dir)
	for _, pdb := range pdbs {
		if !pdb.IsDir() {
			continue
		}
		pdbDir := filepath.Join(r.Dir, pdb.Name())
		keys, _ := os.ReadDir(pdbDir)
		for _, key := range keys {
			keyDir := filepath.Join(pdbDir, key.Name())
			marker := filepath.Join(keyDir, missMarker)
			if info, err := os.Stat(marker); err == nil && time.Since(info.ModTime()) > missMaxAge {
				_ = os.Remove(marker)
			}
			if info, err := os.Stat(keyDir); err == nil && info.IsDir() && time.Since(info.ModTime()) > partMaxAge {
				_ = os.Remove(keyDir)
			}
		}
		_ = os.Remove(pdbDir)
	}
}
