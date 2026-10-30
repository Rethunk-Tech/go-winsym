# HUMANS.md

Run and use `go-winsym`. Internals: [AGENTS.md](AGENTS.md).

## Quick start

```bash
go get github.com/Rethunk-Tech/go-winsym
```

```go
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	winsym "github.com/Rethunk-Tech/go-winsym"
)

func main() {
	f, err := os.Open("crash.dmp")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		fmt.Println(err)
		return
	}
	fault, err := winsym.ParseMinidump(f, info.Size())
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("exception %#x in %s +%#x\n", fault.Code, fault.Module, fault.Offset)

	// A Resolver downloads into Dir, which it must own. With no Wait set it
	// answers from the cache or not at all, so this example does no network I/O.
	r := &winsym.Resolver{Dir: filepath.Join(os.TempDir(), "winsym-symbols")}
	if name, off, ok := r.Name(context.Background(), fault.CodeView, fault.Offset); ok {
		fmt.Printf("%s +%#x\n", name, off)
	}
}
```

## Contracts

| Entry point | Contract |
| --- | --- |
| `ParseMinidump` | Returns an error for a non-dump, a truncated one or one without exception and module streams; never panics. `Module` is the path as the dump records it, empty when no listed module holds the address. |
| `Fault.CodeView`, `PECodeView` | Zero when no valid record exists. A non-empty `PDB` is a base name matching `^[A-Za-z0-9_.-]+\.pdb$`. |
| `CodeView.Key` | The symbol-store folder: GUID with its first three fields little-endian, upper-case hex, then the age in hex. |
| `Lookup` | Pure: reads only the `io.ReaderAt` given. Names the public function at or below the address; only `?Name@Scope@@`, `??0` and `??1` are undecorated, any other name is returned as stored. |
| `Resolver.Name` | `ok` is false when no `Server` serves the PDB, it is neither cached nor fetchable now, or no symbol covers the address. Never panics on a nil `HTTP`. |
| `Resolver.Dir` | Layout `<Dir>/<pdb>/<key>/<pdb>`. Must be this `Resolver`'s alone: pruning deletes `*.pdb` and stale `*.pdb.part` files under it. |
| `Resolver.Online` | False means no network; a cached PDB is still read. |
| `Resolver.Servers` | Asked in order; a nil `Serves` asks for every name. Nil `Servers` means `DefaultServers()`. |
| Failed downloads | A miss is remembered for seven days only when every server answered 404; any other failure is retried on the next call. |
| Download and cache size | One download is capped, and the cache is held under `MaxCacheBytes` by dropping the least recently read PDBs. |
| Download wait | `Name` waits at most `Wait` or its context; the download continues for the next call. |

## Requirements

Go 1.27.3 or newer, plus `lefthook` and `gitleaks` on PATH if you want the git hooks.

```bash
lefthook install
```

That is per clone; git does not commit hooks. Pre-commit scans staged changes for secrets, and
pre-push runs `gate`.

## Verify

```bash
gate
```
