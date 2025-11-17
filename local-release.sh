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

# Push to OCI
echo "Pushing to OCI registry..."
for file in dist/terraform-provider-forgejo_$VERSION*.zip; do
  filename=$(basename "$file")
  echo "Pushing $filename..."

  oras push $REGISTRY/$GITHUB_USER/$PROVIDER_NAME:$VERSION \
    --artifact-type application/vnd.opentofu.provider.layer.v1+zip \
    $file:application/zip
done

echo "✓ Successfully pushed to $REGISTRY/$GITHUB_USER/$PROVIDER_NAME:$VERSION"
