#!/bin/bash

set -o errexit -o nounset -o pipefail

export GOCACHE="$PWD/gocache"
export GOMODCACHE="$PWD/gocache"

: "${GCS_RESOURCE_JSON_KEY:?}"
: "${GCS_RESOURCE_BUCKET_NAME:?}"
: "${GCS_RESOURCE_VERSIONED_BUCKET_NAME:?}"

make -C gcs-resource integration-tests
