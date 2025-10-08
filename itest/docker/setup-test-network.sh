#!/bin/bash

# This file sets up a test Lightning Network with multiple nodes and channels
# using Docker. It creates a network topology, opens channels, performs
# payments, and force closes some channels to prepare all node state for the
# integration tests.

# DIR is set to the directory of this script.
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

source "$DIR/compose.sh"
source "$DIR/network.sh"
source "$DIR/.env"

cd $DIR

# Prepare all folders.
export NODES=(alice bob charlie dave rusty nifty snyke chantools)

shopt -s dotglob
for node in "${NODES[@]}"; do
  mkdir -p "$DIR/node-data/${node}"
  rm -rf "$DIR/node-data/${node}/"{*,.*}
done

# Spin up the network in detached mode.
compose_up

# Set up the basic A ◄─► B ◄─► C ◄─► D network.
#                  └────►└► R ◄┘
#                           └► N
setup_bitcoin

wait_for_nodes alice bob charlie dave rusty nifty snyke

do_for fund_node alice bob charlie dave rusty nifty snyke

# Alice, Bob and Charlie will open more than one channel each.
do_for fund_node alice alice bob charlie

mine 6

wait_for_nodes rusty nifty snyke

connect_nodes alice bob
connect_nodes alice rusty
connect_nodes alice nifty
connect_nodes alice snyke
connect_nodes bob charlie
connect_nodes bob rusty
connect_nodes bob nifty
connect_nodes charlie dave
connect_nodes charlie rusty
connect_nodes rusty nifty

open_channel alice bob
open_channel alice rusty
open_channel bob charlie
open_channel charlie dave
open_channel bob rusty
open_channel charlie rusty
open_channel rusty nifty
open_channel alice snyke

echo "🔗  Set up network: Alice ◄─► Bob ◄─► Charlie ◄─► Dave network."
echo "                     └────────►└► Rusty ◄┘ "
echo "                     |             └► Nifty"
echo "                     └► Snyke"

mine 12

num_channels=8

wait_graph_sync alice $num_channels
wait_graph_sync bob $num_channels
wait_graph_sync charlie $num_channels
wait_graph_sync dave $num_channels

# Test several multi-hop payments.
send_payment bob dave
send_payment dave bob
send_payment alice dave
send_payment alice rusty
send_payment dave rusty
send_payment alice snyke

# Repeat the basic tests.
send_payment bob dave
send_payment dave bob
send_payment alice dave
send_payment alice rusty
send_payment dave rusty
send_payment alice snyke

# Store all the channel information in separate JSON files.
alice listchannels > "$DIR/node-data/chantools/alice-channels.json"
bob listchannels > "$DIR/node-data/chantools/bob-channels.json"
charlie listchannels > "$DIR/node-data/chantools/charlie-channels.json"
dave listchannels > "$DIR/node-data/chantools/dave-channels.json"
rusty listchannels > "$DIR/node-data/chantools/rusty-channels.json"

# Store all the node pubkeys in a single file.
echo "alice: " $(node_identity alice) >> "$DIR/node-data/chantools/identities.txt"
echo "bob: " $(node_identity bob) >> "$DIR/node-data/chantools/identities.txt"
echo "charlie: " $(node_identity charlie) >> "$DIR/node-data/chantools/identities.txt"
echo "dave: " $(node_identity dave) >> "$DIR/node-data/chantools/identities.txt"
echo "rusty: " $(node_identity rusty) >> "$DIR/node-data/chantools/identities.txt"
echo "nifty: " $(node_identity nifty) >> "$DIR/node-data/chantools/identities.txt"
echo "snyke: " $(node_identity snyke) >> "$DIR/node-data/chantools/identities.txt"

# Force close the two channels of Bob-Charlie and Charlie-Rusty.
RUSTY=$( node_identity rusty )
CHARLIE=$( node_identity charlie )
CHAN_BOB_CHARLIE=$(bob listchannels | jq -r '.channels[] | select(.remote_pubkey=="'$CHARLIE'") | .channel_point')
CHAN_ALICE_RUSTY=$(alice listchannels | jq -r '.channels[] | select(.remote_pubkey=="'$RUSTY'") | .channel_point')
echo "🛡️  Force closing Bob-Charlie channel: $CHAN_BOB_CHARLIE"
echo "🛡️  Force closing Alice-Rusty channel: $CHAN_ALICE_RUSTY"
bob closechannel --force --chan_point "$CHAN_BOB_CHARLIE"
alice closechannel --force --chan_point "$CHAN_ALICE_RUSTY"

# Revert the changes that are made in lncli to change some of the field names.
sed -i.BAK 's/"chan_id"/"channel_id"/g' "$DIR/node-data/chantools/alice-channels.json"
sed -i.BAK 's/"chan_id"/"channel_id"/g' "$DIR/node-data/chantools/bob-channels.json"
sed -i.BAK 's/"chan_id"/"channel_id"/g' "$DIR/node-data/chantools/charlie-channels.json"
sed -i.BAK 's/"chan_id"/"channel_id"/g' "$DIR/node-data/chantools/dave-channels.json"
sed -i.BAK 's/"scid"/"chan_id"/g' "$DIR/node-data/chantools/alice-channels.json"
sed -i.BAK 's/"scid"/"chan_id"/g' "$DIR/node-data/chantools/bob-channels.json"
sed -i.BAK 's/"scid"/"chan_id"/g' "$DIR/node-data/chantools/charlie-channels.json"
sed -i.BAK 's/"scid"/"chan_id"/g' "$DIR/node-data/chantools/dave-channels.json"
rm "$DIR/node-data/chantools/"*.json.BAK

# We stop all the nodes except for Dave and Snyke, which are used for the
# triggerforceclose sub command test.
compose_stop alice
compose_stop bob
compose_stop charlie
compose_stop rusty
compose_stop nifty

# Mine the force closed channels, after shutting down the nodes, to prevent them
# from automatically sweeping the funds.
mine 1

echo "🛡️ ⚔️ 🫡   Test setup created successfully!  🫡 ⚔️ 🛡️"
