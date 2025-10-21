#!/bin/bash

# The run-cmd.sh file can be used to call any helper functions directly from the
# command line to call any function defined in compose.sh or network.sh.
# For example:
#   $ ./run-cmd.sh compose-up

# DIR is set to the directory of this script.
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

source "$DIR/.env"
source "$DIR/compose.sh"
source "$DIR/network.sh"

CMD=$1
shift
$CMD "$@"
