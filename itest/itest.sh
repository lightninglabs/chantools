#!/bin/bash

# This script sets up the dockerized LN cluster and runs the integration tests.

# Stop the script if an error is returned by any step.
set -e

# ITEST_DIR is set to the directory of this script.
ITEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

source "$ITEST_DIR/docker/compose.sh"
source "$ITEST_DIR/docker/network.sh"
source "$ITEST_DIR/docker/.env"

# We make sure the cluster is always stopped in case it was left running from a
# previous run.
compose_down || true

# Ensure that the cluster is shut down when the script errors out due to the
# set -e flag above.
trap compose_down ERR

# If the user doesn't intend to debug the tests (i.e. by keeping the containers
# running after the tests), then we ensure that the cluster is torn down at the
# end of the script.
if [[ -z "$DEBUG" ]] && [[ -z "$debug" ]]; then
  trap compose_down EXIT
else
  echo "⚠️  Debug mode enabled, not stopping cluster at the end of the tests."
fi

"$ITEST_DIR"/docker/setup-test-network.sh

# The network setup part was successful, we don't need that trap anymore.
trap - ERR

# Make sure we're in the itest directory before running the tests.
cd "$ITEST_DIR"

# Run the integration tests.
go test -v -count=1 -test.run ^TestIntegration${CASE} ./...
