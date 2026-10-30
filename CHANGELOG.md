# Changelog

Notable changes to `go-winsym`. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `Lookup` names the public function at or below an address in a Windows PDB read from any `io.ReaderAt`.
- `PECodeView` reads the PDB name, GUID and age a PE file records, and `CodeView.Key` gives the symbol-store folder for them.
- `Server` and `DefaultServers` describe which symbol servers are asked for which PDB names.

[Unreleased]: https://github.com/Rethunk-Tech/go-winsym/commits/main
