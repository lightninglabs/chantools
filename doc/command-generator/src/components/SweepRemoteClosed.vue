<template>
  <div class="card">
    <div class="card-body">
      <h2 class="card-title">SweepRemoteClosed command</h2>
      <p class="lead">
        {{command.description}}
      </p>
      <form @submit.prevent="generateArgs" @change="generateArgs">
        <div class="mb-3">
          <label for="sweepaddr" class="form-label">Sweep address</label>
          <input
              type="text"
              class="form-control"
              id="sweepaddr"
              v-model="args.sweepaddr"
              :class="{ 'is-invalid': !args.sweepaddr }"
              @change="generateArgs"
              @keyup="generateArgs"
          />
          <div class="form-text">
            The address to send any recovered funds to; specify 'fromseed' to
            derive a new address from the seed automatically.
          </div>
        </div>
        <div class="mb-3">
          <label for="feerate" class="form-label">Fee rate (sat/vByte)</label>
          <input
            type="number"
            class="form-control"
            id="feerate"
            v-model="args.feerate"
            @change="generateArgs"
            @keyup="generateArgs"
          />
          <div class="form-text">
            The fee rate to use for the sweep transaction in sat/vByte.
          </div>
        </div>
        <div class="mb-3 form-check">
          <input
              type="checkbox"
              class="form-check-input"
              id="publish"
              v-model="args.publish"
              @change="generateArgs"
          />
          <label class="form-check-label" for="publish">
            Publish transaction automatically
          </label>
          <div class="form-text">
            If the created sweep transaction should be published automatically
            to the chain API instead of just printing the TX.
          </div>
        </div>
        <div class="mb-3">
          <label for="recoverywindow" class="form-label">Recovery window</label>
          <input
              type="number"
              class="form-control"
              id="recoverywindow"
              v-model="args.recoverywindow"
              @change="generateArgs"
          />
          <div class="form-text">
            The number of keys to scan per derivation path. Adjust to match the
            approximate number of channels the node had during its lifetime.
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
          <div class="mb-3">
            <label for="known_outputs" class="form-label">Known outputs</label>
            <input
              type="text"
              class="form-control"
              id="known_outputs"
              v-model="args.known_outputs"
              @change="generateArgs"
              @keyup="generateArgs"
            />
            <div class="form-text">
              A comma separated list of known output addresses to use for matching
              against, instead of querying the API; can also be a file name to a
              file that contains the known outputs, one per line.
            </div>
          </div>
          <div class="mb-3">
            <label for="peers" class="form-label">Peer public keys</label>
            <input
              type="text"
              class="form-control"
              id="peers"
              v-model="args.peers"
              @change="generateArgs"
              @keyup="generateArgs"
            />
            <div class="form-text">
              comma separated list of hex encoded public keys of the remote peers
              to recover funds from, only required when using --hsm_secret to
              derive the keys; can also be a file name to a file that contains the
              public keys, one per line
            </div>
          </div>
        </div>
      </form>
    </div>
  </div>
</template>

<script>
const defaultValues = {
  feerate: 30,
  recoverywindow: 200,
};

export default {
  name: 'SweepRemoteClosed',
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
        feerate: defaultValues.feerate,
        known_outputs: '',
        peers: '',
        publish: defaultValues.publish,
        recoverywindow: defaultValues.recoverywindow,
        sweepaddr: ''
      },
    };
  },
  methods: {
    generateArgs() {
      let args = {};
      for (const [key, value] of Object.entries(this.args)) {
        // We skip default values to make the commands shorter.
        if (defaultValues[key] && value === defaultValues[key]) {
          continue;
        }

        if (value !== '') {
          args[key] = value;
        }
      }
      this.$emit('generate-args', args);
    },
  },
};
</script>
