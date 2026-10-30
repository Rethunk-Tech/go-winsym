# Changelog

Notable changes to `go-winsym`. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `Lookup` names the public function at or below an address in a Windows PDB read from any `io.ReaderAt`.
- `PECodeView` reads the PDB name, GUID and age a PE file records, and `CodeView.Key` gives the symbol-store folder for them.
- `Server` and `DefaultServers` describe which symbol servers are asked for which PDB names.
- `Resolver.Name` fetches a PDB once into a size-capped, symbol-store-layout cache and names the function an address lies in, retrying on Windows while a scanner holds the file.
- `ParseMinidump` reads the exception code, address, faulting module path, offset and CodeView record from a Windows minidump without dbghelp.

[Unreleased]: https://github.com/Rethunk-Tech/go-winsym/commits/main
