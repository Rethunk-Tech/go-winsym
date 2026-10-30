// Package winsym resolves native Windows crashes to function names: it reads the faulting module and offset from a
// minidump, a module's CodeView record from a dump or a PE file, fetches the matching PDB from a symbol server into a
// size-capped cache, and names the public function an address lies in by reading the PDB's MSF container.
package winsym
