#!/usr/bin/env bash
set -eux -o pipefail

PROVIDER_NAME="forgejo"
GITHUB_USER="kfkonrad"
REGISTRY="ghcr.io"

# Build with GoReleaser (no tags needed)
echo "Building with GoReleaser..."
goreleaser release --snapshot --clean --skip=publish,sign

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
