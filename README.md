<h1 align="center">go-winsym</h1>

<div align="center">

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.27.3+-00ADD8?logo=go&logoColor=white)](go.mod)

</div>

---

Symbol resolution for native Windows crashes: read minidumps, find PDBs on symbol servers and name the function an address lies in. Package `winsym`, module `github.com/Rethunk-Tech/go-winsym`.

It works without dbghelp or Windows, on files and servers it does not trust, using only the standard library. Every format it reads is Microsoft's and documented.

## Quick start

```bash
go get github.com/Rethunk-Tech/go-winsym
```

A worked example and the contracts: [HUMANS.md](HUMANS.md).

## Features

- Minidump reader: exception code, address, faulting module and offset, and the module's CodeView record.
- CodeView from a PE file on disk, with the symbol-store key for its PDB.
- PDB lookup: the public function at or below an address, partly undecorated.
- Symbol-server fetch into a size-capped cache in the standard symbol-store layout, downloaded once however many callers wait.
- Crash discovery (`crashdump`): the minidumps WER and Unity crash folders leave, Application Error and .NET faults from the Windows event log, and systemd core dumps, each as one `Fault` naming the module, offset and exception.
- Server lists you choose, so a module's name goes only to servers that publish it.

## Documentation

| Doc | Audience |
| --- | --- |
| [HUMANS.md](HUMANS.md) | Running and using the package |
| [AGENTS.md](AGENTS.md) | Internals, invariants, and why |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Process — commits, tests, docs |
| [SECURITY.md](SECURITY.md) | Vulnerability disclosure |
| [CHANGELOG.md](CHANGELOG.md) | Release notes |

## License

MIT — see [LICENSE](LICENSE). © 2026 Rethunk.Tech, LLC.
