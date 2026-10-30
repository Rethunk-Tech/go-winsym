#!/usr/bin/env bash
# Rebuilds ../pe-codeview.dll and ../msf-publics.pdb, the PE image and Windows PDB the tests map addresses in, with clang-cl
# and lld's link flavour (rust-lld, which rustup ships, or lld-link).
# The DLL records the PDB name "UnityPlayer_Win64_player_mono_x64.pdb", a name a default server serves; the tests rely on it.
set -euo pipefail
cd "$(dirname "$0")"
out=$(mktemp -d "${TMPDIR:-/var/tmp}/winsym-fixture.XXXXXX")
trap 'rm -rf "$out"' EXIT
link=$(command -v lld-link || find "$HOME/.rustup/toolchains" -name rust-lld -path '*stable*' | head -n 1)
flavour=()
[[ $link == *rust-lld ]] && flavour=(-flavor link)
clang-cl --target=x86_64-pc-windows-msvc /c /Z7 /Od /GS- /Fo"$out/crash.obj" crash.cpp
"$link" "${flavour[@]}" /dll /debug /noentry /nodefaultlib /pdbaltpath:%_PDB% /out:"$out/UnityPlayer.dll" \
  /pdb:"$out/UnityPlayer_Win64_player_mono_x64.pdb" "$out/crash.obj"
cp "$out/UnityPlayer.dll" ../pe-codeview.dll
cp "$out/UnityPlayer_Win64_player_mono_x64.pdb" ../msf-publics.pdb
