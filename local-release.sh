#!/usr/bin/env bash
set -eux -o pipefail

PROVIDER_NAME="forgejo"
GITHUB_USER="kfkonrad"
REGISTRY="ghcr.io"

# Parse arguments
#   (none)                    snapshot build, version <last tag>-SNAPSHOT-<commit>
#   --release                 release build, needs a git tag on HEAD
#   --prerelease <version>    snapshot build published as <version>, e.g. 0.14.0-beta.1;
#                             no git tag needed, so nothing reaches the public registries
SNAPSHOT_FLAG="--snapshot"
PRERELEASE_VERSION=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --release) SNAPSHOT_FLAG="" ;;
    --prerelease) PRERELEASE_VERSION="${2#v}"; shift ;;
    *) echo "Unknown argument: $1" >&2; exit 1 ;;
  esac
  shift
done

if [[ -n "$PRERELEASE_VERSION" ]]; then
  if [[ -z "$SNAPSHOT_FLAG" ]]; then
    echo "--release and --prerelease are mutually exclusive" >&2
    exit 1
  fi
  if ! [[ "$PRERELEASE_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+-[0-9A-Za-z.-]+$ ]]; then
    echo "Pre-release version must look like 1.2.3-beta.1, got: $PRERELEASE_VERSION" >&2
    exit 1
  fi
  # Read by snapshot.version_template in .goreleaser.yml
  export PRERELEASE_VERSION
  echo "Building pre-release $PRERELEASE_VERSION..."
elif [[ -z "$SNAPSHOT_FLAG" ]]; then
  echo "Building non-snapshot release..."
else
  echo "Building snapshot..."
fi

# Build with GoReleaser (no tags needed)
echo "Building with GoReleaser..."
goreleaser release $SNAPSHOT_FLAG --clean --skip=publish,sign

# Get the version from metadata
VERSION=$(cat dist/metadata.json | jq -r .version)
echo "Version: $VERSION"

# Login to GHCR
echo "Logging in to GHCR..."
set +x
echo $GITHUB_TOKEN | oras login ghcr.io -u $GITHUB_USER --password-stdin
set -x

# Push each platform as a separate manifest and collect references
MANIFESTS=()

echo "Pushing platform-specific manifests..."
for file in dist/terraform-provider-forgejo_$VERSION*.zip; do
  filename=$(basename "$file")

  # Extract platform info from filename
  # e.g., terraform-provider-forgejo_v0.5.4-SNAPSHOT-xxx_linux_amd64.zip
  if [[ $filename =~ _([a-z]+)_([a-z0-9]+)\.zip$ ]]; then
    OS="${BASH_REMATCH[1]}"
    ARCH="${BASH_REMATCH[2]}"

    echo "Pushing $OS/$ARCH..."

    # Push individual platform manifest
    DIGEST=$(oras push $REGISTRY/$GITHUB_USER/$PROVIDER_NAME:$VERSION-$OS-$ARCH \
      --format=go-template='{{.digest}}' \
      --artifact-type application/vnd.opentofu.provider-target \
      --artifact-platform $OS/$ARCH \
      $file:archive/zip)

    MANIFESTS+=("$REGISTRY/$GITHUB_USER/$PROVIDER_NAME@$DIGEST")
  fi
done

# Create multi-platform index
echo "Creating multi-platform index..."
oras manifest index create $REGISTRY/$GITHUB_USER/$PROVIDER_NAME:$VERSION \
  --artifact-type application/vnd.opentofu.provider \
  ${MANIFESTS[@]}

echo "✓ Successfully pushed to $REGISTRY/$GITHUB_USER/$PROVIDER_NAME:$VERSION"
