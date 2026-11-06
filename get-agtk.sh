#!/bin/sh
# Installs agtk (Linux and macOS) to $HOME/.agtk/bin/agtk. No sudo, no root, no Go needed.
# It downloads the release archive, checks it against SHA256SUMS, and prints the PATH line
# for your shell; it never edits your shell startup files.
#
#   curl -fsSL https://raw.githubusercontent.com/willlovecode/agent-config/master/get-agtk.sh | sh
#
# Environment:
#   AGTK_VERSION   release to install, e.g. 1.2.3 or v1.2.3 (default: the latest release)
#   AGTK_BASE_URL  folder holding SHA256SUMS and the archives (default: the GitHub release)
#   AGTK_FILE      install this archive instead of downloading (offline or private repo)
#   AGTK_SUMS      SHA256SUMS file to verify AGTK_FILE against (without it AGTK_FILE is unverified)
set -eu

repo=willlovecode/agent-config

say() { printf '%s\n' "$*"; }
die() { printf 'get-agtk: %s\n' "$*" >&2; exit 1; }

[ -n "${HOME:-}" ] || die "HOME is not set"

case $(uname -s) in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) die "unsupported system $(uname -s). On Windows use get-agtk.ps1." ;;
esac
case $(uname -m) in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) die "unsupported CPU $(uname -m): releases exist for x86_64 and aarch64 (arm64)" ;;
esac

tmp=$(mktemp -d "${TMPDIR:-/tmp}/get-agtk.XXXXXX") || die "cannot create a temporary folder"
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

fetch() { # url file
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL --retry 2 -o "$2" "$1"
  elif command -v wget >/dev/null 2>&1; then
    wget -q -O "$2" "$1"
  else
    die "need curl or wget to download (or set AGTK_FILE to an archive you downloaded)"
  fi
}

sha256_of() { # file
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d ' ' -f 1
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | cut -d ' ' -f 1
  fi
}

# verify archive sumsfile: the archive's checksum must match its line in sumsfile.
verify() {
  command -v sha256sum >/dev/null 2>&1 || command -v shasum >/dev/null 2>&1 ||
    die "need sha256sum or shasum to verify the archive"
  name=$(basename "$1")
  want=$(awk -v n="$name" '{ f = $2; sub(/^\*/, "", f); if (f == n) { print $1; exit } }' "$2")
  [ -n "$want" ] || die "$name is not listed in $2"
  have=$(sha256_of "$1")
  [ "$have" = "$want" ] || die "checksum mismatch for $name (expected $want, got $have); not installing"
  say "Checksum ok: $name"
}

if [ -n "${AGTK_FILE:-}" ]; then
  archive=$AGTK_FILE
  [ -f "$archive" ] || die "AGTK_FILE $archive does not exist"
  if [ -n "${AGTK_SUMS:-}" ]; then
    [ -f "$AGTK_SUMS" ] || die "AGTK_SUMS $AGTK_SUMS does not exist"
    verify "$archive" "$AGTK_SUMS"
  else
    say "Warning: AGTK_SUMS not set, so $archive is not verified."
  fi
else
  if [ -n "${AGTK_BASE_URL:-}" ]; then
    base=${AGTK_BASE_URL%/}
  elif [ -n "${AGTK_VERSION:-}" ]; then
    base=https://github.com/$repo/releases/download/v${AGTK_VERSION#v}
  else
    base=https://github.com/$repo/releases/latest/download
  fi
  say "Downloading agtk for $os/$arch from $base"
  fetch "$base/SHA256SUMS" "$tmp/SHA256SUMS" ||
    die "cannot download $base/SHA256SUMS. If the repository is private or you are offline, download the archive by hand and run: AGTK_FILE=<archive> AGTK_SUMS=<SHA256SUMS> sh get-agtk.sh"
  # The asset name carries the version, so find it in the checksum list.
  name=$(awk -v s="_${os}_${arch}.tar.gz" '{ f = $2; sub(/^\*/, "", f); if (length(f) > length(s) && substr(f, length(f) - length(s) + 1) == s) { print f; exit } }' "$tmp/SHA256SUMS")
  [ -n "$name" ] || die "no $os/$arch archive listed in $base/SHA256SUMS"
  archive=$tmp/$name
  fetch "$base/$name" "$archive" || die "cannot download $base/$name"
  verify "$archive" "$tmp/SHA256SUMS"
fi

mkdir "$tmp/x"
tar -xzf "$archive" -C "$tmp/x" agtk || die "cannot extract agtk from $archive"
[ -f "$tmp/x/agtk" ] || die "$archive does not contain an agtk binary"

bindir=$HOME/.agtk/bin
mkdir -p "$bindir" || die "cannot create $bindir"
# Copy next to the target, then rename: a running agtk can be replaced, and a failed copy leaves the old one.
cp "$tmp/x/agtk" "$bindir/.agtk.new.$$" || die "cannot write to $bindir"
chmod 755 "$bindir/.agtk.new.$$"
mv -f "$bindir/.agtk.new.$$" "$bindir/agtk"

if version=$("$bindir/agtk" version 2>/dev/null); then
  say "Installed agtk $version to $bindir/agtk"
else
  say "Installed $bindir/agtk, but it did not run here. Is this a noexec mount, or the wrong CPU?"
fi

case ":${PATH:-}:" in
  *":$bindir:"*)
    say "$bindir is already on your PATH."
    cmd=agtk
    ;;
  *)
    cmd=$bindir/agtk
    say ""
    say "$bindir is not on your PATH yet. Add it (this script does not edit your shell files):"
    case $(basename "${SHELL:-sh}") in
      fish) say "  fish_add_path \"\$HOME/.agtk/bin\"" ;;
      zsh) say "  echo 'export PATH=\"\$HOME/.agtk/bin:\$PATH\"' >> ~/.zshrc" ;;
      bash)
        rc=.bashrc
        [ "$os" = darwin ] && rc=.bash_profile
        say "  echo 'export PATH=\"\$HOME/.agtk/bin:\$PATH\"' >> ~/$rc"
        ;;
      *) say "  export PATH=\"\$HOME/.agtk/bin:\$PATH\"   # and put the same line in your shell's startup file" ;;
    esac
    say "then open a new terminal."
    ;;
esac

say ""
say "Next: $cmd wizard"
