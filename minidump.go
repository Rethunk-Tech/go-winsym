package winsym

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"unicode/utf16"
)

// Fault is where a minidump's exception happened.
type Fault struct {
	// Code is the exception code, such as 0xc0000005.
	Code    uint32
	Address uint64
	// Module is the faulting image's path as the dump records it, empty when no listed module holds Address.
	Module string
	// Offset is Address minus that module's base; meaningful only when Module is not empty.
	Offset uint64
	// CodeView is the module's debug record, zero when the dump carries no valid one.
	CodeView CodeView
}

const (
	minidumpMagic    = 0x504d444d // "MDMP"
	moduleListStream = 4
	exceptionStream  = 6
	moduleEntrySize  = 108
	maxStreams       = 64
	maxModules       = 4096
	maxNameBytes     = 1 << 12
)

var errNotMinidump = errors.New("not a minidump")

// ParseMinidump finds the faulting module of a Windows minidump: the exception stream gives the address and the
// module list says which loaded image holds it. The dump is another program's file: counts, offsets and sizes in it
// are bounded before they size an allocation or a read.
func ParseMinidump(r io.ReaderAt, size int64) (Fault, error) {
	var head [32]byte
	if _, err := r.ReadAt(head[:], 0); err != nil {
		return Fault{}, errNotMinidump
	}
	le := binary.LittleEndian
	if le.Uint32(head[0:]) != minidumpMagic {
		return Fault{}, errNotMinidump
	}
	count, dirRva := le.Uint32(head[8:]), le.Uint32(head[12:])
	if count == 0 || count > maxStreams {
		return Fault{}, fmt.Errorf("minidump lists %d streams", count)
	}
	var excRva, modRva uint32
	var excSize uint32
	for i := range count {
		var d [12]byte
		if _, err := r.ReadAt(d[:], int64(dirRva)+int64(i)*12); err != nil {
			return Fault{}, err
		}
		switch le.Uint32(d[0:]) {
		case exceptionStream:
			excSize, excRva = le.Uint32(d[4:]), le.Uint32(d[8:])
		case moduleListStream:
			modRva = le.Uint32(d[8:])
		}
	}
	if excRva == 0 || excSize < 32 || modRva == 0 {
		return Fault{}, errors.New("minidump has no exception or module list")
	}
	var exc [32]byte
	if _, err := r.ReadAt(exc[:], int64(excRva)); err != nil {
		return Fault{}, err
	}
	fault := Fault{Code: le.Uint32(exc[8:]), Address: le.Uint64(exc[24:])}

	var n [4]byte
	if _, err := r.ReadAt(n[:], int64(modRva)); err != nil {
		return Fault{}, err
	}
	modules := min(le.Uint32(n[:]), maxModules)
	for i := range modules {
		var m [moduleEntrySize]byte
		if _, err := r.ReadAt(m[:], int64(modRva)+4+int64(i)*moduleEntrySize); err != nil {
			return Fault{}, err
		}
		base, length := le.Uint64(m[0:]), uint64(le.Uint32(m[8:]))
		if fault.Address < base || fault.Address-base >= length {
			continue
		}
		name, err := minidumpString(r, size, le.Uint32(m[20:]))
		if err != nil {
			return Fault{}, err
		}
		fault.Module, fault.Offset = name, fault.Address-base
		if cvSize, cvRva := le.Uint32(m[76:]), le.Uint32(m[80:]); cvSize <= maxCodeView && int64(cvRva)+int64(cvSize) <= size {
			raw := make([]byte, cvSize)
			if _, err := r.ReadAt(raw, int64(cvRva)); err == nil {
				fault.CodeView, _ = parseCodeView(raw)
			}
		}
		break
	}
	return fault, nil
}

// minidumpString reads a MINIDUMP_STRING: a byte length, then UTF-16LE text.
func minidumpString(r io.ReaderAt, size int64, rva uint32) (string, error) {
	var n [4]byte
	if _, err := r.ReadAt(n[:], int64(rva)); err != nil {
		return "", err
	}
	length := binary.LittleEndian.Uint32(n[:])
	if length > maxNameBytes || int64(rva)+4+int64(length) > size {
		return "", errors.New("minidump string is out of range")
	}
	raw := make([]byte, length)
	if _, err := r.ReadAt(raw, int64(rva)+4); err != nil {
		return "", err
	}
	units := make([]uint16, len(raw)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(raw[i*2:])
	}
	return string(utf16.Decode(units)), nil
}
