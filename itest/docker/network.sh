#!/bin/bash

# This file contains all LN network related functions.

# DIR is set to the directory of this script.
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

source "$DIR/.env"

function node_identity() {
    local node="$1"
    local key
    if [[ "$node" =~ ^($CLN_NODES)$ ]]; then
        key=$( $node getinfo | jq .id -r)
    else
        key=$( $node getinfo | jq .identity_pubkey -r)
    fi
    echo "$key"
}

# wait_for_nodes waits for all the nodes in the argument list to start.
function wait_for_nodes() {
  local nodes=("$@")

  for node in "${nodes[@]}"; do
    wait_for_node $node
  done
  echo "🏎️  All nodes have started!"
}

# wait_for_node waits for the given node in the cluster to start, with a
# timeout.
function wait_for_node() {
  if [[ $# -ne 1 ]]; then
      echo "❌  Error: wait_for_node requires exactly 1 argument (node)"
      echo "Usage: wait_for_node <node>"
      return 1
  fi

  local node="$1"
  local start_time=$(date +%s)

  echo -n "⌛  Waiting for $node to start (timeout: ${TIMEOUT}s)"

  if [[ "$node" =~ ^($CLN_NODES)$ ]]; then
      while ! $node getinfo | grep -q \"id\"; do
          sleep 1
      done
      BLOCKS=$(bitcoin getblockchaininfo | jq .blocks -r)
      while [[ $($node getinfo | jq .blockheight -r | xargs) -lt $BLOCKS ]]; do
          sleep 1
      done
      
      echo
      echo "✅  $node has started"
        
      return 0
  fi

  while ! $node state 2>/dev/null | grep -q SERVER_ACTIVE; do
      echo -n "."
      sleep 0.5

      # Check if timeout has been reached.
      local elapsed_time=$(( $(date +%s) - start_time ))
      if [[ $elapsed_time -ge $TIMEOUT ]]; then
          echo
          echo "❌  Error: Timeout after $TIMEOUT seconds waiting for $node to start"
          return 1
      fi
  done

  echo
  echo "✅  $node has started"
}

# do_for is a generic function to execute a command for a set of nodes.
function do_for() {
  if [[ $# -lt 2 ]]; then
      echo "❌  Error: do_for requires at least 2 arguments (function and nodes)"
      echo "Usage: do_for <function> [node1] [node2] [node3]..."
      return 1
  fi

  local func="$1"
  shift
  local nodes=("$@")

  for node in "${nodes[@]}"; do
      "$func" "$node"
  done
}

# fund_node funds the specified node with 5 BTC.
function fund_node() {
  local node="$1"

  if [[ "$node" =~ ^($CLN_NODES)$ ]]; then
      ADDR=$( $node newaddr bech32 | jq .bech32 -r)
  else
      ADDR=$( $node newaddress p2wkh | jq .address -r)
  fi

  bitcoin sendtoaddress "$ADDR" 5 > /dev/null

  echo "💰  Funded $node with 5 BTC"
}

# connect_nodes connects two specified nodes.
function connect_nodes() {
  if [[ $# -ne 2 ]]; then
      echo "❌  Error: connect_nodes requires exactly 2 arguments (node1 and node2)"
      echo "Usage: connect_nodes <node1> <node2>"
      return 1
  fi

  local node1="$1"
  local node2="$2"

  echo -ne "📞  Connecting $node1 to $node2...\r"

  KEY_2=$( node_identity $node2 )

  if [[ "$node1" =~ ^($CLN_NODES)$ ]]; then
    $node1 connect "$KEY_2" $node2 9735 > /dev/null
  else
    $node1 connect "$KEY_2"@$node2:9735 > /dev/null
  fi

  echo -ne "                        \r"
  echo "📞  Connected $node1 to $node2"
}

# open_channel opens a channel between two specified nodes.
function open_channel() {
  if [[ $# -ne 2 ]]; then
      echo "❌  Error: open_channel requires exactly 2 arguments (node1 and node2)"
      echo "Usage: open_channel <node1> <node2>"
      return 1
  fi

  local node1="$1"
  local node2="$2"

  KEY_2=$( node_identity $node2 )

  if [[ "$node1" =~ ^($CLN_NODES)$ ]]; then
    $node1 fundchannel "id=$KEY_2" amount=15000000 push_msat=7000000000 > /dev/null
  else
    $node1 openchannel --node_key "$KEY_2" --local_amt 15000000 --push_amt 7000000 > /dev/null
  fi

  echo "🔗  Opened channel between $node1 and $node2"
}

# Function to check if a node's graph has the expected number of channels.
function wait_graph_sync() {
  if [[ $# -ne 2 ]]; then
       echo "❌  Error: graph_synced requires exactly 2 arguments (node and num_chans)"
       echo "Usage: graph_synced <node> <num_chans>"
       return 1
  fi

  local node="$1"
  local num_chans="$2"

  while :; do
    num_channels=$($node getnetworkinfo | jq -r '.num_channels')

    # Ensure num_channels is a valid number before proceeding.
    if [[ "$num_channels" =~ ^[0-9]+$ ]]; then
      echo -ne "⌛  $node sees $num_channels channels...\r"

      if [[ "$num_channels" -eq num_chans ]]; then
        echo "👀  $node sees all the channels!"
        break  # Exit loop when num_channels reaches num_chans.
      fi
    fi

    sleep 1
  done
}

# send_payment attempts to send a payment between two specified nodes.
function send_payment() {
  if [[ $# -ne 2 ]]; then
      echo "❌  Error: send_payment requires exactly 2 arguments (from_node and to_node)"
      echo "Usage: send_payment <from_node> <to_node>"
      return 1
  fi

  local from_node="$1"
  local to_node="$2"

  # Generate invoice and capture error output, then extract payment request.
  local invoice_output
  local PAY_REQ
  
  if [[ "$to_node" =~ ^($CLN_NODES)$ ]]; then
      label=$(openssl rand -hex 12)
      if ! invoice_output=$($to_node invoice 10000 $label "" 2>&1); then
          echo "❌  Error: Failed to generate invoice from $to_node"
          echo "📜 Details: $invoice_output"
          return 1
      fi
      
      PAY_REQ=$(echo "$invoice_output" | jq -r '.bolt11')
  else
      if ! invoice_output=$($to_node addinvoice 10000 2>&1); then
          echo "❌  Error: Failed to generate invoice from $to_node"
          echo "📜 Details: $invoice_output"
          return 1
      fi
      
      PAY_REQ=$(echo "$invoice_output" | jq -r '.payment_request')
  fi

  # Ensure invoice creation was successful.
  if [[ -z "$PAY_REQ" || "$PAY_REQ" == "null" ]]; then
      echo "❌  Error: Invoice response did not contain a valid payment request."
      echo "📜 Raw Response: $invoice_output"
      return 1
  fi

  # Send payment and capture error output.
  local payment_output
  if ! payment_output=$($from_node payinvoice --force "$PAY_REQ" 2>&1); then
      echo "❌  Error: Payment failed from $from_node to $to_node"
      echo "📜 Details: $payment_output"
      return 1
  fi

  echo "💸  Payment sent from $from_node to $to_node"
}

# wait_for_active_chans waits for a node to have the expected number of active
# channels.
function wait_for_active_chans() {
  if [[ $# -ne 2 ]]; then
      echo "❌  Error: wait_for_active_chans requires exactly 2 arguments (node and expected_active_channels)"
      echo "Usage: wait_for_active_chans <node> <num_channels>"
      return 1
  fi

  local node="$1"
  local expected_channels="$2"

  echo "🟠  Waiting for $node to have exactly $expected_channels active channels..."

  while :; do
      # Get the active channel count.
      local active_count
      active_count=$($node --network=regtest listchannels 2>/dev/null | jq '[.channels[] | select(.active == true)] | length')

      # Ensure active_count is a valid number.
      if [[ "$active_count" =~ ^[0-9]+$ ]]; then
          echo -ne "⌛ $node sees $active_count active channels...\r"

          # Exit loop only if the expected number of channels is active.
          if [[ "$active_count" -eq "$expected_channels" ]]; then
              break
          fi
      fi

      sleep 1
  done

  echo
  echo "🟢  $node now has exactly $expected_channels active channels!"
}

# mine mines a number of blocks on the regtest network. If no argument is
# provided, it defaults to 6 blocks.
function mine() {
  NUMBLOCKS="${1-6}"
  bitcoin generatetoaddress "$NUMBLOCKS" "$(bitcoin getnewaddress "" legacy)" > /dev/null
}

# setup_bitcoin performs various operations on the regtest bitcoind node so that
# it is ready to be used by the Lightning nodes and so that it can be used to
# fund the nodes.
function setup_bitcoin() {
  echo "🔗  Setting up Bitcoin node"
  bitcoin createwallet miner  > /dev/null

  ADDR_BTC=$(bitcoin getnewaddress "" legacy)
  bitcoin generatetoaddress 106 "$ADDR_BTC" > /dev/null
  bitcoin getbalance > /dev/null

  echo "🔗  Bitcoin node is set up"
}

function bitcoin() {
  docker exec -i -u bitcoin bitcoind bitcoin-cli -regtest -rpcuser=lightning -rpcpassword=lightning "$@"
}

function alice() {
  docker exec -i alice lncli --network regtest "$@"
}

function bob() {
  docker exec -i bob lncli --network regtest "$@"
}

function charlie() {
  docker exec -i charlie lncli --network regtest "$@"
}

function dave() {
  docker exec -i dave lncli --network regtest "$@"
}

function rusty() {
  docker exec -i rusty lightning-cli --network regtest "$@"
}

function nifty() {
  docker exec -i nifty lightning-cli --network regtest "$@"
}

function snyke() {
  docker exec -i snyke lightning-cli --network regtest "$@"
}
