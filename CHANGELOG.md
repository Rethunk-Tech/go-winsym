# Changelog

Notable changes to `go-winsym`. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `crashdump.Windows` finds the minidumps Windows Error Reporting and Unity crash folders left for a game's executables, and `crashdump.ReadDump` reads one.
- `crashdump.EventLog` and `ParseEventLog` read Application Error and .NET unhandled-exception faults from the Windows event log.
- `crashdump.Coredump` and `ParseCoredumpInfo` read a systemd core dump report, and `ParseUnityErrorLog` a Unity `error.log`.
- `crashdump.Fault` names the module, offset, exception code, time, dump path and source of a crash.

## [0.1.0] - 2026-10-29

### Added

- `Lookup` names the public function at or below an address in a Windows PDB read from any `io.ReaderAt`.
- `PECodeView` reads the PDB name, GUID and age a PE file records, and `CodeView.Key` gives the symbol-store folder for them.
- `Server` and `DefaultServers` describe which symbol servers are asked for which PDB names.
- `Resolver.Name` fetches a PDB once into a size-capped, symbol-store-layout cache and names the function an address lies in, retrying on Windows while a scanner holds the file.
- `ParseMinidump` reads the exception code, address, faulting module path, offset and CodeView record from a Windows minidump without dbghelp.

[Unreleased]: https://github.com/Rethunk-Tech/go-winsym/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/Rethunk-Tech/go-winsym/releases/tag/v0.1.0
