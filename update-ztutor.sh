#!/usr/bin/env bash
# update-ztutor.sh — Download and install the latest release of ztutor/ztutord.
#
# Usage:
#   ./update-ztutor.sh                # normal update
#   ./update-ztutor.sh --dir /opt/bin # custom install directory
#   ./update-ztutor.sh --dry-run      # check what would be installed
#   ./update-ztutor.sh --force        # install same version
#   ./update-ztutor.sh --help         # this message
#
# The script compares the currently installed version against the latest
# GitHub release.  It stops the systemd ztutord service (if running),
# replaces the binaries, and restarts the service.
set -euo pipefail

REPO="Zuhaitz-dev/ztutor"
INSTALL_DIR="/usr/local/bin"
SERVICE_NAME="ztutord"
DRY_RUN=false
FORCE=false

while [ $# -gt 0 ]; do
    case "$1" in
        --dir) INSTALL_DIR="$2"; shift 2 ;;
        --dry-run) DRY_RUN=true; shift ;;
        --force) FORCE=true; shift ;;
        --help) sed -n '/^# /p' "$0" | head -10 | sed 's/^# //'; exit 0 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

# ── Detect current version ──────────────────────────────────────────────
CURRENT=""
ZTUTOR_BIN="${INSTALL_DIR}/ztutor"
if [ -x "$ZTUTOR_BIN" ]; then
    CURRENT=$("$ZTUTOR_BIN" --version 2>/dev/null | grep -oP 'ztutor \K\S+') || CURRENT=""
fi
[ -z "$CURRENT" ] && CURRENT="none"

# ── Fetch latest release info ───────────────────────────────────────────
echo "Checking latest release from github.com/$REPO ..."
LATEST_JSON=$(curl -sf "https://api.github.com/repos/$REPO/releases/latest") || {
    echo "ERROR: Failed to fetch release info. Check your internet connection."
    exit 1
}

LATEST=$(echo "$LATEST_JSON" | grep -oP '"tag_name": "\K[^"]+')
PUBLISHED=$(echo "$LATEST_JSON" | grep -oP '"published_at": "\K[^"]+' | cut -dT -f1)
RELEASE_URL="https://github.com/$REPO/releases/tag/$LATEST"

echo "  Current: ${CURRENT}"
echo "  Latest:  ${LATEST} (${PUBLISHED})"

if [ "$CURRENT" = "$LATEST" ] && [ "$FORCE" = false ]; then
    echo "Already up to date."
    exit 0
fi

# ── Detect platform ─────────────────────────────────────────────────────
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "ERROR: Unsupported architecture: $ARCH"; exit 1 ;;
esac

TARBALL="ztutor-${LATEST}-${OS}-${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/$REPO/releases/download/$LATEST/$TARBALL"

echo "  Download: $TARBALL"
echo "  Platform: ${OS}/${ARCH}"
echo "  Details:  $RELEASE_URL"
echo ""

if [ "$DRY_RUN" = true ]; then
    echo "Dry-run mode. Run without --dry-run to install."
    exit 0
fi

# ── Confirm ─────────────────────────────────────────────────────────────
read -r -p "Install ${LATEST} to ${INSTALL_DIR}? [y/N] " CONFIRM
case "$CONFIRM" in
    [yY]|[yY][eE][sS]) ;;
    *) echo "Aborted."; exit 0 ;;
esac

# ── Download and extract ────────────────────────────────────────────────
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

echo "Downloading ${TARBALL} ..."
curl -#L -o "${TMPDIR}/${TARBALL}" "$DOWNLOAD_URL"

echo "Extracting ..."
tar -xzf "${TMPDIR}/${TARBALL}" -C "$TMPDIR"

# ── Stop service ────────────────────────────────────────────────────────
if command -v systemctl >/dev/null 2>&1; then
    if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        echo "Stopping ${SERVICE_NAME} ..."
        sudo systemctl stop "$SERVICE_NAME"
    fi
fi

# ── Install binaries ────────────────────────────────────────────────────
echo "Installing to ${INSTALL_DIR} ..."
sudo install -m 755 "${TMPDIR}/ztutor"  "${INSTALL_DIR}/ztutor"
sudo install -m 755 "${TMPDIR}/ztutord" "${INSTALL_DIR}/ztutord"
sudo install -m 755 "${TMPDIR}/update-ztutor.sh" "${INSTALL_DIR}/update-ztutor.sh" 2>/dev/null || true

# ── Restart service ─────────────────────────────────────────────────────
if command -v systemctl >/dev/null 2>&1; then
    if systemctl list-units --full -all 2>/dev/null | grep -q "$SERVICE_NAME"; then
        echo "Starting ${SERVICE_NAME} ..."
        sudo systemctl start "$SERVICE_NAME"
        echo "Service ${SERVICE_NAME} restarted."
    fi
fi

echo ""
echo "Update complete: ${CURRENT} -> ${LATEST}"
echo "Run 'ztutor --version' to verify."
