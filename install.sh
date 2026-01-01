#!/bin/sh
# install.sh - Build, install, and configure basar
#
# Usage:
#   ./install.sh              # Install to ~/.local/bin + full setup
#   ./install.sh /usr/local   # Install to /usr/local/bin (needs sudo)
#   ./install.sh --no-setup   # Install only, skip setup
#
set -e

PREFIX=""
NO_SETUP=""

# Parse arguments
for arg in "$@"; do
    case "$arg" in
        --no-setup)
            NO_SETUP=1
            ;;
        *)
            PREFIX="$arg"
            ;;
    esac
done

PREFIX="${PREFIX:-$HOME/.local}"
BINDIR="$PREFIX/bin"

cd "$(dirname "$0")"

# Check for Go
if ! command -v go >/dev/null 2>&1; then
    echo "error: go is required but not found" >&2
    exit 1
fi

echo "Building basar..."
CGO_ENABLED=0 go build -ldflags="-s -w" -o basar ./cmd/basar

echo "Installing to $BINDIR..."
mkdir -p "$BINDIR"

if [ -w "$BINDIR" ]; then
    cp basar "$BINDIR/"
    chmod 755 "$BINDIR/basar"
else
    echo "Need elevated privileges for $BINDIR"
    sudo cp basar "$BINDIR/"
    sudo chmod 755 "$BINDIR/basar"
fi

rm -f basar

# Ensure BINDIR is in PATH for setup
export PATH="$BINDIR:$PATH"

echo ""
echo "basar installed to $BINDIR"

# Run setup unless --no-setup was given
if [ -z "$NO_SETUP" ]; then
    echo ""
    echo "Running setup..."
    echo ""
    "$BINDIR/basar" --setup -v
    echo ""
    echo "Setup complete! Just run:"
    echo "  volatility3 -f memory.dmp linux.pslist"
    echo ""
    echo "The cache auto-updates every 2 weeks via systemd timer."
else
    echo ""
    echo "To complete setup later, run:"
    echo "  basar --setup"
    echo ""
    echo "Or manually:"
    echo "  volatility3 -u \$(basar) -f memory.dmp linux.pslist"
fi
