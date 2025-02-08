#!/bin/bash
# bootstrap-wrapper.sh
#
# This script downloads the correct bootstrap binary for the current machine's OS and architecture
# from GitHub Releases, performs pre-flight checks, makes it executable, and then executes it.
#
# Usage:
#   curl -sSL https://your-domain.com/bootstrap-wrapper.sh | bash -s -- [options] -- [arguments to bootstrap binary]
#
# Options:
#   -h, --help          Show this help message and exit.
#   -v, --verbose       Enable verbose output.
#   --role ROLE         Specify the role for provisioning.
#   --mise-install      Enable one-shot systemd service for 'mise install' after reboot.
#   --list-roles        Query the GitHub API and print the list of available roles, then exit.
#
# Exit immediately if a command exits with a non-zero status,
# and treat unset variables as an error.
set -euo pipefail

usage() {
  cat <<EOF
Usage: $0 [options] -- [arguments to bootstrap binary]

Options:
  -h, --help          Show this help message and exit.
  -v, --verbose       Enable verbose output.
  --role ROLE         Specify the role for provisioning.
  --mise-install      Enable one-shot systemd service for 'mise install' after reboot.
  --list-roles        Query the GitHub API and print the list of available roles, then exit.
EOF
  exit 0
}

# Default values.
VERBOSE="false"
ROLE="base"
MISE_INSTALL="false"
LIST_ROLES="false"

# Parse wrapper options.
while [[ $# -gt 0 ]]; do
  case "$1" in
  -h | --help)
    usage
    ;;
  -v | --verbose)
    VERBOSE="true"
    shift
    ;;
  --role)
    if [[ $# -gt 1 ]]; then
      ROLE="$2"
      shift 2
    else
      echo "Error: --role requires an argument."
      usage
    fi
    ;;
  --mise-install)
    MISE_INSTALL="true"
    shift
    ;;
  --list-roles)
    LIST_ROLES="true"
    shift
    ;;
  --)
    shift
    break
    ;;
  *)
    # Stop processing options on first unknown argument.
    break
    ;;
  esac
done

if [ "$VERBOSE" == "true" ]; then
  echo "Wrapper options:"
  echo "  VERBOSE: $VERBOSE"
  echo "  ROLE: $ROLE"
  echo "  MISE_INSTALL: $MISE_INSTALL"
  echo "  LIST_ROLES: $LIST_ROLES"
  echo "Arguments for bootstrap binary: $@"
fi

# If the user requested the list of available roles, query GitHub API and exit.
if [ "$LIST_ROLES" == "true" ]; then
  echo "Querying available roles from GitHub..."
  API_URL="https://api.github.com/repos/sparkleHazard/ansible/contents/roles"
  if command -v jq >/dev/null 2>&1; then
    ROLES=$(curl -sSL "$API_URL" | jq -r '.[] | select(.type=="dir") | .name' | sort | tr '\n' ', ')
    echo "Available roles: ${ROLES%, }"
  else
    echo "Error: jq is required to list roles. Please install jq and try again."
    exit 1
  fi
  exit 0
fi

# Determine the OS type.
OS_TYPE=$(uname -s)
varOS=""
if [ "$OS_TYPE" = "Darwin" ]; then
  varOS="darwin"
elif [ "$OS_TYPE" = "Linux" ]; then
  varOS="linux"
else
  echo "Error: Unsupported OS type: $OS_TYPE"
  exit 1
fi

# Determine the architecture and map x86_64 to amd64.
ARCH=$(uname -m)
case "$ARCH" in
x86_64)
  ARCH="amd64"
  ;;
arm64 | aarch64)
  ARCH="arm64"
  ;;
armv7l)
  ARCH="armv7l"
  ;;
armv6l)
  ARCH="armv6l"
  ;;
*)
  echo "Error: Unsupported architecture: $ARCH"
  exit 1
  ;;
esac

echo "Detected OS: $varOS"
echo "Detected architecture: $ARCH"

# Build the binary URL.
# Assuming your GitHub Releases use the naming convention: bootstrap-<os>-<arch>
BINARY_URL="https://github.com/sparkleHazard/bootstrap/releases/latest/download/bootstrap-${varOS}-${ARCH}"

# Destination for the downloaded binary.
DEST="/tmp/bootstrap"

echo "Downloading bootstrap binary from ${BINARY_URL}..."
curl -sSL "$BINARY_URL" -o "$DEST"

if [ ! -f "$DEST" ]; then
  echo "Error: Failed to download bootstrap binary."
  exit 1
fi

# Optionally, add checksum verification here.
echo "Making bootstrap binary executable..."
chmod +x "$DEST"

echo "Launching bootstrap binary..."
# Execute the binary with any arguments passed to the wrapper.
exec "$DEST" "$@"
