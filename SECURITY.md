# Security

## Supported versions

The latest tagged release. There are no long-lived release branches, so a fix ships in the next tag
rather than being backported.

## Reporting a vulnerability

Use GitHub's **private vulnerability reporting** on this repository (Security → Report a
vulnerability). It stays private until a fix ships.

Please do not open a public issue for a suspected vulnerability first.

## What is in scope

- `ParseMinidump` reads a dump another program wrote: it must not panic, read outside the dump or allocate more than its fixed stream, module, name and CodeView bounds allow, however the counts and offsets lie.
- `PECodeView` reads a PE file at a path: a crafted debug directory must not panic or allocate beyond the bound on its size and record.
- `Lookup` reads a PDB a symbol server, or anyone on the path of a plain-http address, could answer: a crafted container must not panic or make it allocate beyond the directory and stream bounds or past the file's size.
- `crashdump` reads dump files, Unity crash folders, event-log XML and `coredumpctl` output: files open through a root on their own folder (a symlink or `..` cannot leave it), dumps over 4 GiB are skipped, `error.log` and the event stream are capped at 1 MiB and 2 MiB, a managed stack is cut at 16 KiB, and an exe name with a path in it is ignored.
- `CodeView.PDB` is the one string from those inputs that becomes a path and a URL: when non-empty it matches `^[A-Za-z0-9_.-]+\.pdb$`, so it cannot hold a folder or traversal.
- `Resolver` downloads from servers: a response must not exceed the download cap however its `Content-Length` reads, and nothing it writes may leave the cache folder.

## What is out of scope

- The caller chooses the paths it hands `PECodeView`, the `Dir` a `Resolver` owns and the `Servers` it trusts; a `Server` list that names a hostile host is a configuration choice. A server on plain http can serve a wrong PDB; the cache is not authenticated, and a wrong PDB yields a wrong name, never code execution.
- Names returned from a PDB are text from the input and are the caller's to display safely.
- A hand-placed file in `Dir` for a module no server serves is ignored by design.
- Time spent on the network and disk by a slow server is bounded by the download timeout and `Wait`, not by a rate limit.
