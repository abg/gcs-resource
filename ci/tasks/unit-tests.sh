#!/bin/bash

set -o errexit -o nounset -o pipefail

export GOCACHE="$PWD/gocache"
export GOMODCACHE="$PWD/gocache"

apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y unzip

make -C gcs-resource
