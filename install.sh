#!/usr/bin/env bash
# HMP installer — clones the source from GitHub, builds it locally,
# sets up this device's identity, installs the binary to PATH,
# then removes all leftover build files (including itself).
set -e

REPO_URL="https://github.com/lghthans/hmp.git"
INSTALL_NAME="hmp"
CLONE_DIR="$(mktemp -d)"

echo "  HMP installer"
echo "  ─────────────"

# --- 1. Make sure git is available ---
if ! command -v git >/dev/null 2>&1; then
  echo "  git not found — installing..."
  if command -v pkg >/dev/null 2>&1; then
    pkg install -y git
  elif command -v apt-get >/dev/null 2>&1; then
    sudo apt-get update && sudo apt-get install -y git
  else
    echo "  ! could not install git automatically — please install it and re-run"
    exit 1
  fi
fi

# --- 2. Make sure Go is available ---
if ! command -v go >/dev/null 2>&1; then
  echo "  go not found — installing..."
  if command -v pkg >/dev/null 2>&1; then
    pkg install -y golang
  elif command -v apt-get >/dev/null 2>&1; then
    sudo apt-get update && sudo apt-get install -y golang-go
  else
    echo "  ! could not install go automatically — please install Go 1.22+ and re-run"
    exit 1
  fi
fi

echo "  ✓ git and go are available"

# --- 3. Clone the source ---
echo "  downloading source from $REPO_URL..."
git clone --quiet --depth 1 "$REPO_URL" "$CLONE_DIR"

# --- 4. Build the binary ---
echo "  building..."
(cd "$CLONE_DIR" && go build -o "$INSTALL_NAME" .)

# --- 5. Determine install target directory (must be on PATH) ---
if [ -n "$PREFIX" ] && [ -d "$PREFIX/bin" ]; then
  # Termux sets $PREFIX (e.g. /data/data/com.termux/files/usr)
  BIN_DIR="$PREFIX/bin"
else
  # Regular Linux
  BIN_DIR="/usr/local/bin"
fi

# --- 6. Move the built binary into place ---
if [ -w "$BIN_DIR" ]; then
  mv "$CLONE_DIR/$INSTALL_NAME" "$BIN_DIR/$INSTALL_NAME"
else
  echo "  requesting elevated permission to install into $BIN_DIR..."
  sudo mv "$CLONE_DIR/$INSTALL_NAME" "$BIN_DIR/$INSTALL_NAME"
fi

echo "  ✓ hmp installed to $BIN_DIR/$INSTALL_NAME"

# --- 7. Clean up the cloned source, we only needed the compiled binary ---
rm -rf "$CLONE_DIR"

# --- 8. First-run setup: choose device name, generate identity ---
echo ""
echo "  Now let's set up this device."
"$BIN_DIR/$INSTALL_NAME" -setup

echo ""
echo "  ✓ installation complete — try running: hmp -status"

# --- 9. Delete this installer script itself ---
rm -- "$0"
