// Package crashdump finds what a native crash left behind (a Windows minidump, a Unity crash folder, the Windows event
// log, a systemd coredump report) and reads it just far enough to name the module the process died in. Every source
// is another program's file or output, so each read is size-capped and stays inside the folder it was asked to read.
package crashdump
