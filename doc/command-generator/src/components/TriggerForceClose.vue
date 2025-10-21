<template>
  <div class="card">
    <div class="card-body">
      <h2 class="card-title">TriggerForceClose</h2>
      <p class="lead">
        {{command.description}}
      </p>
      <form @submit.prevent="generateArgs" @change="generateArgs">
        <div class="mb-3">
          <label for="channel_point" class="form-label">
            Channel Point
          </label>
          <input
            type="text"
            class="form-control"
            id="channel_point"
            v-model="args.channel_point"
            @change="generateArgs"
            @keyup="generateArgs"
          />
          <div class="form-text">
            The funding transaction outpoint of the channel to trigger the force
            close of (&lt;txid&gt;:&lt;txindex&gt;).
          </div>
        </div>
        <div class="mb-3">
          <label for="peer" class="form-label">Peer URI</label>
          <input
            type="text"
            class="form-control"
            id="peer"
            v-model="args.peer"
            @change="generateArgs"
            @keyup="generateArgs"
          />
          <div class="form-text">
            The remote peer's full address (&lt;pubkey&gt;@&lt;host&gt;[:&lt;port&gt;]).
          </div>
        </div>
        <div class="mb-3">
          <label for="torproxy" class="form-label">Tor proxy address</label>
          <input
            type="text"
            class="form-control"
            id="torproxy"
            v-model="args.torproxy"
            @change="generateArgs"
            @keyup="generateArgs"
          />
          <div class="form-text">
            SOCKS5 proxy to use for Tor connections (to .onion addresses).
            Usually this is <code>localhost:9050</code> if a Tor daemon is
            running on the same machine as <code>chantools</code>.
          </div>
        </div>

        <div class="mb-3">
          <button class="btn btn-secondary" type="button"
                  data-bs-toggle="collapse"
                  data-bs-target="#advancedOptions">
            Show advanced options
          </button>
        </div>
        <div class="collapse" id="advancedOptions">
          <div class="mb-3 form-check">
            <input
                type="checkbox"
                class="form-check-input"
                id="all_public_channels"
                v-model="args.all_public_channels"
                @change="generateArgs"
            />
            <label class="form-check-label" for="all_public_channels">
              Attempt to close all public channels
            </label>
            <div class="form-text">
              Query all public channels from the Amboss API and attempt to
              trigger a force close for each of them.
            </div>
          </div>
        </div>
      </form>
    </div>
  </div>
</template>

<script>

export default {
  name: 'TriggerForceClose',
  emits: ['generate-args'],
  props: {
    command: {
      type: Object,
      required: true,
    },
  },
  data() {
    return {
      args: {
        all_public_channels: false,
        channel_point: '',
        peer: '',
        torproxy: '',
      },
    };
  },
  methods: {
    generateArgs() {
      this.$emit('generate-args', this.args);
    },
  },
};
</script>
