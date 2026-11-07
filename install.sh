#!/usr/bin/env bash

# install.sh - Speedgrapher Installer
# 1. Downloads prebuilt release binary (via GoReleaser) or explicitly compiles via 'go install' (--build)
# 2. Automatically executes 'speedgrapher install' to configure MCP server and Agent Skills

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m' # No Color

REPO="KaiLepler/speedgrapher"
VERSION="latest"
BUILD_FROM_SOURCE="false"
INIT_ARGS=()

print_usage() {
  cat << 'EOF'
Speedgrapher Installer

Usage:
  install.sh [components] [options]
  curl -fsSL https://raw.githubusercontent.com/KaiLepler/speedgrapher/main/install.sh | bash -s -- [components] [options]

Components (default: all):
  --mcp                    Register the MCP server in mcp_config.json
  --skills                 Unpack embedded skills (@deslopify, @inverted-pyramid, @tech-interviewer, @tech-writer, @tech-reviewer, @tech-publisher)

Options:
  -g, --global             Initialize in global user config (Default: ~/.gemini/config)
  -w, --workspace          Initialize in workspace scope (.agents/)
  -v, --version <v>        Target Speedgrapher release version (Default: latest)
      --build              Build from source via 'go install' instead of prebuilt binary
  -f, --force              Force overwrite of existing skill files
  -q, --quiet              Quiet / script-friendly output
  -c, --config <path>      Explicit path to mcp_config.json
  -s, --skills-dir <dir>   Explicit directory for skills installation
  -h, --help               Show this help message

Examples:
  ./install.sh                      # Install binary + configure MCP & Skills globally
  ./install.sh -w                   # Install binary + configure MCP & Skills in workspace (.agents/)
  ./install.sh --mcp                # Install binary + configure MCP server only
  ./install.sh --skills             # Install binary + configure Skills only
  ./install.sh --build              # Explicitly build from source via 'go install'
EOF
}

# Parse installer arguments
while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)
      print_usage
      exit 0
      ;;
    --build)
      BUILD_FROM_SOURCE="true"
      shift
      ;;
    -v|--version)
      if [[ $# -lt 2 || -z "$2" ]]; then
        printf 'Error: --version requires a value.\n' >&2
        exit 1
      fi
      VERSION="$2"
      shift 2
      ;;
    *)
      INIT_ARGS+=("$1")
      shift
      ;;
  esac
done

echo -e "${BLUE}===============================================${NC}"
echo -e "${BLUE}         Speedgrapher Installer                ${NC}"
echo -e "${BLUE}===============================================${NC}"
echo -e "Version: ${BOLD}${VERSION}${NC}"
echo ""

# Determine target install directory for binary
GOPATH_BIN=""
if command -v go &> /dev/null; then
  GOBIN="$(go env GOBIN)"
  if [ -n "${GOBIN}" ]; then
    INSTALL_BIN_DIR="${GOBIN}"
  else
    INSTALL_BIN_DIR="$(go env GOPATH)/bin"
  fi
else
  INSTALL_BIN_DIR="${HOME}/.local/bin"
fi

mkdir -p "${INSTALL_BIN_DIR}"
BIN_PATH="${INSTALL_BIN_DIR}/speedgrapher"

# 1. Download prebuilt release binary (GoReleaser)
if [ "${BUILD_FROM_SOURCE}" != "true" ]; then
  OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
  ARCH="$(uname -m)"
  case "${ARCH}" in
    x86_64|amd64) ARCH="x64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *) ARCH="" ;;
  esac

  if [ -z "${ARCH}" ] || [[ ! "${OS}" =~ ^(darwin|linux)$ ]]; then
    echo -e "${RED}❌ Error: Unsupported OS (${OS}) or Architecture (${ARCH}).${NC}" >&2
    echo -e "   Please build from source using: ${BOLD}curl -fsSL ... | bash -s -- --build${NC}" >&2
    exit 1
  fi

  echo -e "📦 ${BLUE}[Binary] Fetching prebuilt binary for ${OS}.${ARCH}...${NC}"

  if [ "${VERSION}" = "latest" ]; then
    # Resolve once: archive and checksum must refer to the same immutable tag.
    RELEASE_PAGE="$(curl -fsSL --connect-timeout 10 --max-time 60 -o /dev/null \
      -w '%{url_effective}' "https://github.com/${REPO}/releases/latest")"
    case "${RELEASE_PAGE}" in
      "https://github.com/${REPO}/releases/tag/"*) VERSION="${RELEASE_PAGE##*/}" ;;
      *) printf 'Error: could not resolve latest release tag.\n' >&2; exit 1 ;;
    esac
  fi
  CLEAN_VER="${VERSION#v}"
  if [[ ! "${CLEAN_VER}" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-][[:alnum:].-]+)?$ ]]; then
    printf 'Error: invalid release version: %s\n' "${VERSION}" >&2
    exit 1
  fi

  ASSET="${OS}.${ARCH}.speedgrapher.tar.gz"
  RELEASE_BASE="https://github.com/${REPO}/releases/download/v${CLEAN_VER}"
  RELEASE_URL="${RELEASE_BASE}/${ASSET}"
  TMP_DIR="$(mktemp -d)"
  STAGED_BIN=""
  trap 'rm -rf "${TMP_DIR}"; if [ -n "${STAGED_BIN}" ]; then rm -f "${STAGED_BIN}"; fi' EXIT
  TAR_FILE="${TMP_DIR}/speedgrapher.tar.gz"

  if ! curl -fsSL --connect-timeout 10 --max-time 120 "${RELEASE_URL}" -o "${TAR_FILE}"; then
    echo -e "${RED}❌ Error: Failed to download prebuilt release for ${OS}.${ARCH}.${NC}" >&2
    echo -e "   URL: ${RELEASE_URL}" >&2
    echo "" >&2
    echo -e "   To build from source instead, re-run with: ${BOLD}--build${NC}" >&2
    exit 1
  fi

  curl -fsSL --connect-timeout 10 --max-time 60 "${RELEASE_BASE}/checksums.txt" \
    -o "${TMP_DIR}/checksums.txt"
  EXPECTED="$(awk -v asset="${ASSET}" '$2 == asset || $2 == "*" asset {print $1; n++} END {if (n != 1) exit 1}' "${TMP_DIR}/checksums.txt")"
  if [[ ! "${EXPECTED}" =~ ^[[:xdigit:]]{64}$ ]]; then
    printf 'Error: missing or invalid release checksum.\n' >&2
    exit 1
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    ACTUAL="$(sha256sum "${TAR_FILE}")"
  else
    ACTUAL="$(shasum -a 256 "${TAR_FILE}")"
  fi
  ACTUAL="${ACTUAL%% *}"
  if [ "${ACTUAL}" != "${EXPECTED}" ]; then
    printf 'Error: release checksum mismatch; installed binary was not changed.\n' >&2
    exit 1
  fi

  # Extract only the known binary entry, not arbitrary archive paths.
  if tar -tzf "${TAR_FILE}" bin/speedgrapher >/dev/null 2>&1; then
    BIN_ENTRY="bin/speedgrapher"
  elif tar -tzf "${TAR_FILE}" speedgrapher >/dev/null 2>&1; then
    BIN_ENTRY="speedgrapher"
  else
    printf 'Error: binary not found in release archive.\n' >&2
    exit 1
  fi
  if ! tar -xzf "${TAR_FILE}" -C "${TMP_DIR}" "${BIN_ENTRY}"; then
    echo -e "${RED}❌ Error: Failed to extract release archive.${NC}" >&2
    exit 1
  fi

  if [ ! -f "${TMP_DIR}/${BIN_ENTRY}" ] || [ -L "${TMP_DIR}/${BIN_ENTRY}" ]; then
    printf 'Error: release binary is not a regular file.\n' >&2
    exit 1
  fi

  STAGED_BIN="$(mktemp "${INSTALL_BIN_DIR}/.speedgrapher.XXXXXX")"
  cp "${TMP_DIR}/${BIN_ENTRY}" "${STAGED_BIN}"
  chmod 755 "${STAGED_BIN}"
  mv -f "${STAGED_BIN}" "${BIN_PATH}"
  STAGED_BIN=""
  echo -e "  ${GREEN}✓ Downloaded and installed to ${BIN_PATH}${NC}"
  rm -rf "${TMP_DIR}"

else
  # 2. Build from source (Explicitly requested via --build)
  echo -e "🔨 ${BLUE}[Binary] Compiling from source via 'go install'...${NC}"
  if ! command -v go &> /dev/null; then
    echo -e "${RED}❌ Error: 'go' toolchain is required to build from source.${NC}" >&2
    exit 1
  fi

  INSTALL_TARGET="github.com/${REPO}/cmd/speedgrapher@${VERSION}"
  go install "${INSTALL_TARGET}"
  if [ -f "${BIN_PATH}" ]; then
    echo -e "  ${GREEN}✓ Installed to ${BIN_PATH}${NC}"
  else
    echo -e "${RED}❌ Error: Binary not found at ${BIN_PATH} after go install.${NC}" >&2
    exit 1
  fi
fi

# 3. Check if binary directory is in PATH
if ! command -v speedgrapher &> /dev/null; then
  echo -e "${YELLOW}⚠️  Note: '${INSTALL_BIN_DIR}' is not currently in your \$PATH.${NC}"
  echo "  Add it to your shell profile (~/.zshrc or ~/.bashrc):"
  echo -e "  ${BLUE}export PATH=\"${INSTALL_BIN_DIR}:\$PATH\"${NC}"
  echo ""
fi

# 4. Delegate surface initialization to 'speedgrapher install'
echo -e "⚙️  ${BLUE}[Surfaces] Running 'speedgrapher install'...${NC}"
"${BIN_PATH}" install "${INIT_ARGS[@]+"${INIT_ARGS[@]}"}"
