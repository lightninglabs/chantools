# Chantools Integration Test (itest)

This directory contains the integration test (itest) setup and test cases for
chantools. The test network is created using Docker Compose scripts and consists
of multiple Lightning Network nodes connected in a specific topology to create
multiple channels with non-zero channel update indexes (by sending some payments
around the network).

## Test Network Topology

The network is set up as follows:

```
Alice ◄──► Bob ◄──► Charlie ◄──► Dave
   └───────►└──► Rusty ◄──┘
   |               └► Nifty
   └► Snyke
```

- Channel **Alice** - **Bob**: Remains open, used by `runZombieRecoveryLndLnd`.
- Channel **Alice** - **Rusty**: Is force closed by Alice, used by
  `runSweepRemoteClosedCln`.
- Channel **Alice** - **Snyke**: Remains open, used by
  `runTriggerForceCloseCln`.
- Channel **Bob** - **Charlie**: Is force closed by Bob, used by
  `runSweepRemoteClosedLnd`.
- Channel **Charlie** - **Dave**: Remains open, used by
  `runTriggerForceCloseLnd`.
- Channel **Bob** - **Rusty**: Remains open, used by `runZombieRecoveryLndCln`.
- Channel **Rusty** - **Charlie**: Remains open, used by
  `runZombieRecoveryClnLnd`.
- Channel **Rusty** - **Nifty**: Remains open, used by
  `runZombieRecoveryClnCln`.

All nodes except for Dave and Snyke are stopped before the integration tests
are run.

Multiple channels are opened between the nodes, and several multi-hop payments
are executed to ensure the network is fully operational and synchronized. After
the setup, each node's channel information is exported to a JSON file in the
`node-data/chantools/` directory.

## Running the Integration Tests

To start the integration tests, run the following command from the project root:

```sh
make itest
```

Which executes the script `itest/itest.sh`.

This script will:
- Build and start the Docker-based test network.
- Set up the channels and perform payments between nodes.
- Export channel data for each node.
- Run the Go integration tests against the prepared network.

## Debugging the Test Network

By default, the test network is torn down automatically after the tests finish
or if an error occurs. If you want to keep the Docker containers running for
debugging purposes, set the `DEBUG` environment variable before running the
script:

```sh
make itest DEBUG=1
```

With `DEBUG` set, the containers will not be stopped automatically, allowing you
to inspect the state of the network and containers for troubleshooting and
run the Golang based integration tests manually several times.

---

For more details on the network setup, see `docker/setup-test-network.sh`.
