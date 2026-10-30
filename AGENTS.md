# AGENTS.md

Package `winsym` resolves native Windows crashes to function names: it reads a minidump's faulting module and a module's CodeView record, fetches the matching PDB from a symbol server into a bounded cache, and names the public function an address lies in. Any crash-triage tool, launcher or collector imports it; it imports only the standard library and no consumer package.

## Gate

`gate` is the command. Each override in `.gate.toml` is there because a runner
cannot resolve that check from PATH. The `windows-latest` test job is the only place
Windows-only behaviour runs.
`retry_windows_test.go` is the only OS-specific test file: it drives the sharing-violation retry. Everything else, including the cache's rename and delete behaviour, runs the same tests on both OSes, and the Windows job is where a just-closed or still-open file differs.

## Shape

- Parsers (`Lookup`, `ParseMinidump`, `PECodeView`) read files another program wrote. Every count, offset and size read from the input is bounded before it sizes an allocation or a read; the bounds are named constants beside the parser. A new field read from a format gets a bound in the same change.
- `parseCodeView` is the single path from bytes to a `CodeView`, and it returns the zero value when the PDB name fails `pdbName`. A non-empty `CodeView.PDB` therefore always matches the pattern, so no path or URL built from it can leave the cache folder or the server path. `urls` and `Resolver.Name` still re-check before building one.
- `Resolver.Name` validates the name and consults `Servers` before it builds any path. A PDB placed in `Dir` for a module no server serves is never read; this also stops a module's name reaching a server that does not publish it.
- Downloads go through one `os.Root` opened on the key folder, so a write, rename or remove cannot leave it. `prune` walks and deletes through a root on `Dir` for the same reason.
- The cache layout `<Dir>/<pdb>/<key>/<pdb>`, the `<pdb>.part` temp name and the empty `miss` marker are the standard symbol-store layout and a stored format: other tools read the folder and old caches stay valid. A miss is recorded only when every asked server answered 404, so a network failure retries on the next call.
- `retry` wraps only the final rename and the `.part` removal. A scanner that holds a just-written file makes both fail with `ERROR_ACCESS_DENIED`, `ERROR_SHARING_VIOLATION` or `ERROR_LOCK_VIOLATION`; without the retry the next URL's attempt fails on the same leftover `.part` and no miss is recorded, so the work repeats on every call. `prune` uses plain removal because a failed delete only defers to the next prune. `os.Root.Rename` does not avoid the hold.
- Fetches are shared per PDB path, and a download outlives a caller that stopped waiting (`context.WithoutCancel`), so the next call finds the file.
- Size caps are policy and unexported: one download is capped on bytes read rather than `Content-Length` alone, and `MaxCacheBytes` is the one consumer-set bound. `Dir` belongs to one `Resolver`, since `prune` deletes by suffix.
- `Lookup` is exported as the network-free primitive; `undecorate` handles plain, `??0` and `??1` names and returns every other form as stored.

## Files

| File | Holds |
| --- | --- |
| `doc.go` | Package documentation |
| `msf.go` | MSF container reader and `Lookup`, `undecorate` |
| `symsrv.go` | `CodeView`, `PECodeView`, `Key`, `Server`, `DefaultServers`, URL routing |
| `resolver.go` | `Resolver`: fetch once, cache, prune, miss markers |
| `retry_windows.go` | Retry while another process holds a file |
| `retry_other.go` | Single attempt on other OSes |
| `minidump.go` | `Fault` and `ParseMinidump` |
