<template>
  <div class="card">
    <div class="card-body">
      <h2 class="card-title">Select a command</h2>
      <p class="lead">
        Select a command below that you want to generate the correct arguments
        for. If you don't know what command to run, the default scenario usually
        is the following for users that have a defunct Lightning Network node
        and just want to rescue their funds out of all channels:
      </p>
      <ol>
        <li>
          Run <code>SweepRemoteClosed</code> to collect all funds from
          channels that were already force-closed by the remote party.
        </li>
        <li>
          For any channels that were not yet force-closed, run
          <code>TriggerForceClose</code> to ask the remote peer to force-close
          the channel on your behalf.
        </li>
        <li>
          Wait for the force-close transactions to confirm on-chain.
        </li>
        <li>
          Run <code>SweepRemoteClosed</code> again to collect any funds from
          the channels that were just force-closed and confirmed in the previous
          step.
        </li>
        <li>
          For any remaining channels where neither party has channel data
          anymore, run <code>ZombieRecovery</code> to try and recover funds
          with the cooperation of the remote peer (requires you to be in contact
          with the remote node operator).
        </li>
      </ol>
      <div class="command-list">
        <div
            v-for="command in commands"
            :key="command.name"
            class="command-item"
            @click="selectCommand(command)"
        >
          <div class="command-name">{{ command.name }}</div>
          <div class="command-description">{{ command.description }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
export default {
  name: 'CommandSelector',
  emits: ['command-selected'],
  data() {
    return {
      commands: [
        {
          name: 'SweepRemoteClosed',
          description: 'Scan through all the addresses that could have funds ' +
              'of channels that were force-closed by the remote party. A ' +
              'public block explorer API is queried for each potential ' +
              'address and if any balance is found, all funds are swept to ' +
              'the provided address.',
        },
        {
          name: 'TriggerForceClose',
          description: 'Attempt to connect to a Lightning Network peer and ' +
              'ask them to trigger a force close of the specified channel. ' +
              'Requires the peer to be online, reachable and still have the ' +
              'channel data.',
        },
        {
          name: 'ZombieRecovery',
          description: 'A multi-step process to rescue funds from channels ' +
              'where both nodes have lost their channel data. This requires ' +
              'cooperation from the channel peer.',
        },
      ],
    };
  },
  methods: {
    selectCommand(command) {
      this.$emit('command-selected', command);
    },
  },
};
</script>

<style scoped>
.command-list {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.command-item {
  display: flex;
  flex-direction: row;
  align-items: center;
  padding: 1rem;
  border: 1px solid #ccc;
  border-radius: 5px;
  cursor: pointer;
  transition: background-color 0.3s;
}

.command-item:hover {
  background-color: #f5f5f5;
}

.command-name {
  font-weight: bold;
  min-width: 200px;
  padding-right: 1rem;
}

.command-description {
  flex-grow: 1;
}
</style>
