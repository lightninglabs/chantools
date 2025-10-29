<template>
  <div class="card">
    <div class="card-body">
      <h2 class="card-title">ZombieRecovery</h2>
      <p class="lead">{{ command.description }}</p>

      <div class="accordion" id="zombieAccordion">
        <!-- Step 1: Create/obtain match file -->
        <div class="accordion-item">
          <h2 class="accordion-header" id="headingOne">
            <button class="accordion-button"
                    type="button"
                    data-bs-toggle="collapse"
                    data-bs-target="#collapseOne"
                    aria-expanded="true"
                    aria-controls="collapseOne">
              Step 1: Create or obtain match file
            </button>
          </h2>
          <div id="collapseOne" class="accordion-collapse collapse show"
               aria-labelledby="headingOne" data-bs-parent="#zombieAccordion">
            <div class="accordion-body">
              <p>
                The first step is to obtain a match file. If you haven't been
                contacted by your peer or a service like
                <a href="https://node-recovery.com" target="_blank">
                  node-recovery.com
                </a>,
                you can create a match file yourself if you have the required
                information.
              </p>
              <form @submit.prevent="downloadMatchFile">
                <div class="mb-3">
                  <label for="node1key" class="form-label">
                    Node 1 public key
                  </label>
                  <input type="text" class="form-control" id="node1key"
                         :class="{ 'is-invalid': !matchFile.node1.identity_pubkey }"
                         v-model="matchFile.node1.identity_pubkey">
                  <div class="form-text">
                    The 66-character hex public key of Node 1.
                  </div>
                </div>
                <div class="mb-3">
                  <label for="node1contact" class="form-label">
                    Node 1 alias or contact info
                  </label>
                  <input type="text" class="form-control" id="node1contact"
                         v-model="matchFile.node1.contact">
                  <div class="form-text">
                    Optional informational text, for user reference only.
                  </div>
                </div>
                <div class="mb-3">
                  <label for="node2key" class="form-label">
                    Node 2 public key
                  </label>
                  <input type="text" class="form-control" id="node2key"
                         :class="{ 'is-invalid': !matchFile.node2.identity_pubkey }"
                         v-model="matchFile.node2.identity_pubkey">
                  <div class="form-text">
                    The 66-character hex public key of Node 2.
                  </div>
                </div>
                <div class="mb-3">
                  <label for="node2contact" class="form-label">
                    Node 2 alias or contact info
                  </label>
                  <input type="text" class="form-control" id="node2contact"
                         v-model="matchFile.node2.contact">
                  <div class="form-text">
                    Optional informational text, for user reference only.
                  </div>
                </div>
                <div class="mb-3">
                  <label for="chanpoint" class="form-label">
                    Channel Point
                  </label>
                  <input type="text"
                         class="form-control"
                         id="chanpoint"
                         :class="{ 'is-invalid': !matchFile.channels[0].chan_point }"
                         @keyup="error = ''"
                         v-model="matchFile.channels[0].chan_point">
                  <div class="form-text">
                    The <code>&lt;txid:outnum&gt;</code> as shown on Lightning
                    Network explorers.
                  </div>
                </div>
                <div class="alert alert-danger" role="alert" v-if="error">
                  {{ error }}
                </div>
                <button type="submit" class="btn btn-primary">
                  Create and Download Match File
                </button>
              </form>
            </div>
          </div>
        </div>

        <!-- Step 2: Prepare Keys -->
        <div class="accordion-item">
          <h2 class="accordion-header" id="headingTwo">
            <button class="accordion-button collapsed" type="button"
                    data-bs-toggle="collapse" data-bs-target="#collapseTwo"
                    aria-expanded="false" aria-controls="collapseTwo">
              Step 2: Prepare Keys (for both peers)
            </button>
          </h2>
          <div id="collapseTwo" class="accordion-collapse collapse"
               aria-labelledby="headingTwo" data-bs-parent="#zombieAccordion">
            <div class="accordion-body">
              <p>
                Both channel peers must run this command. You need to provide
                the match file from the previous step and a payout address where
                your funds will be sent.<br />
                The person running the &quot;Make Offer&quot; step (next step)
                will need both output files generated by this command from both
                peers.
              </p>
              <form>
                <div class="mb-3">
                  <label for="match_file_prepare" class="form-label">
                    Match file
                  </label>
                  <input type="text"
                         class="form-control"
                         id="match_file_prepare"
                         v-model="prepareKeys.match_file"
                         placeholder="path/to/match-0xxxxxxx-0yyyyyyy.json">
                  <div class="form-text">
                    The full directory/filename of the file on the machine you
                    run <code>chantools</code> on.
                  </div>
                </div>
                <div class="mb-3">
                  <label for="payout_addr" class="form-label">
                    Payout address
                  </label>
                  <input type="text"
                         class="form-control"
                         id="payout_addr"
                         v-model="prepareKeys.payout_addr"
                         placeholder="bc1q...">
                  <div class="form-text">
                    The Bitcoin on-chain address where <b>YOUR</b> part of the
                    channel funds should be sent when the channel is closed.
                  </div>
                </div>
              </form>
              <div class="generated-command mt-4">
                <h3>Generated Command:</h3>
                <pre class="alert alert-secondary"><code>{{
                    generatedPrepareKeysCommand
                  }}</code></pre>
              </div>
            </div>
          </div>
        </div>

        <!-- Step 3: Make Offer -->
        <div class="accordion-item">
          <h2 class="accordion-header" id="headingThree">
            <button class="accordion-button collapsed" type="button"
                    data-bs-toggle="collapse" data-bs-target="#collapseThree"
                    aria-expanded="false" aria-controls="collapseThree">
              Step 3: Make Offer (one peer)
            </button>
          </h2>
          <div id="collapseThree" class="accordion-collapse collapse"
               aria-labelledby="headingThree" data-bs-parent="#zombieAccordion">
            <div class="accordion-body">
              <p>
                After both peers have prepared their keys, one peer creates an
                offer to split the funds. You will need the two files generated
                by the <code>zombierecovery preparekeys</code> command from both
                peers.
              </p>
              <form>
                <div class="mb-3">
                  <label for="node1_keys" class="form-label">
                    Node 1 prepared keys file
                  </label>
                  <input type="text" class="form-control"
                         id="node1_keys"
                         v-model="makeOffer.node1_keys"
                         placeholder="path/to/preparedkeys-yyyy-mm-dd-0xxxxxxx.json">
                  <div class="form-text">
                    The full directory/filename of the file on the machine you
                    run <code>chantools</code> on. This is either the file
                    generated by you or your peer in the previous step.
                  </div>
                </div>
                <div class="mb-3">
                  <label for="node2_keys" class="form-label">
                    Node 2 prepared keys file
                  </label>
                  <input type="text" class="form-control"
                         id="node2_keys"
                         v-model="makeOffer.node2_keys"
                         placeholder="path/to/preparedkeys-yyyy-mm-dd-0yyyyyyy.json">
                  <div class="form-text">
                    The full directory/filename of the file on the machine you
                    run <code>chantools</code> on. This is either the file
                    generated by you or your peer in the previous step.
                  </div>
                </div>
                <div class="mb-3">
                  <label for="feerate_makeoffer" class="form-label">
                    Fee rate
                  </label>
                  <input type="number"
                         class="form-control"
                         id="feerate_makeoffer"
                         v-model="makeOffer.feerate">
                  <div class="form-text">
                    The fee rate to use for the closing transaction in
                    sat/vByte.
                  </div>
                </div>
              </form>
              <div class="generated-command mt-4">
                <h3>Generated Command:</h3>
                <pre class="alert alert-secondary"><code>{{
                    generatedMakeOfferCommand }}</code></pre>
              </div>
            </div>
          </div>
        </div>

        <!-- Step 4: Sign Offer -->
        <div class="accordion-item">
          <h2 class="accordion-header" id="headingFour">
            <button class="accordion-button collapsed" type="button"
                    data-bs-toggle="collapse" data-bs-target="#collapseFour"
                    aria-expanded="false" aria-controls="collapseFour">
              Step 4: Sign Offer (other peer)
            </button>
          </h2>
          <div id="collapseFour" class="accordion-collapse collapse"
               aria-labelledby="headingFour" data-bs-parent="#zombieAccordion">
            <div class="accordion-body">
              <p>
                The peer that did not create the offer now inspects and signs
                it.<br />
                The output of the <code>chantools makeoffer</code> command is a
                PSBT that needs to be provided here.
              </p>
              <form>
                <div class="mb-3">
                  <label for="psbt" class="form-label">
                    Offer PSBT (base64)
                  </label>
                  <textarea class="form-control"
                            id="psbt"
                            rows="3"
                            v-model="signOffer.psbt">
                  </textarea>
                  <div class="form-text">
                    The offer as created by the <code>chantools makeoffer</code>
                    command in the previous step. Be careful when copying this
                    text to not add any extra spaces or newlines and not miss
                    any characters.
                  </div>
                </div>
                <div class="mb-3" v-if="globalArgs.command.hsm_secret">
                  <label for="node1key" class="form-label">
                    Remote peer public key
                  </label>
                  <input type="text" class="form-control" id="node1key"
                         :class="{ 'is-invalid': !signOffer.remote_peer }"
                         v-model="signOffer.remote_peer">
                  <div class="form-text">
                    The 66-character hex public key of the peer node. Required
                    for CLN only.
                  </div>
                </div>
                <div class="mb-3 form-check">
                  <input type="checkbox"
                         class="form-check-input"
                         id="publish"
                         v-model="signOffer.publish">
                  <label class="form-check-label" for="publish">
                    Publish transaction
                  </label>
                  <div class="form-text">
                    Automatically send the signed transaction to the chain API
                    instead of just printing the TX.
                  </div>
                </div>
              </form>
              <div class="generated-command mt-4">
                <h3>Generated Command:</h3>
                <pre class="alert alert-secondary"><code>{{
                    generatedSignOfferCommand }}</code></pre>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
function commandString(globalArgs, subCommand, pageArgs) {
  let commandName = 'zombierecovery';

  if (!globalArgs.preCommand || !globalArgs.command) {
    return `chantools ${commandName} ${subCommand}`;
  }

  let globalArgsString = '';
  for (const [key, value] of Object.entries(globalArgs.preCommand)) {
    if (value) {
      if (typeof value === 'boolean') {
        globalArgsString += `--${key} `;
      } else {
        globalArgsString += `--${key} ${value} `;
      }
    }
  }
  let commandArgs = '';
  for (const [key, value] of Object.entries(globalArgs.command)) {
    if (value) {
      if (typeof value === 'boolean') {
        commandArgs += `--${key} `;
      } else {
        commandArgs += `--${key} ${value} `;
      }
    }
  }
  for (const [key, value] of Object.entries(pageArgs)) {
    if (value) {
      if (typeof value === 'boolean') {
        commandArgs += `--${key} `;
      } else {
        commandArgs += `--${key} ${value} `;
      }
    }
  }

  return `chantools ${globalArgsString}${commandName} ${subCommand} ${commandArgs.trim()}`;
}

export default {
  name: 'ZombieRecovery',
  props: {
    command: {
      type: Object,
      required: true,
    },
    globalArgs: {
      type: Object,
      default: () => ({
        preCommand: {},
        command: {},
      }),
    },
  },
  data() {
    return {
      matchFile: {
        node1: {identity_pubkey: '', contact: ''},
        node2: {identity_pubkey: '', contact: ''},
        channels: [{chan_point: '', capacity: 0, address: ''}],
      },
      prepareKeys: {
        match_file: '',
        payout_addr: '',
      },
      makeOffer: {
        node1_keys: '',
        node2_keys: '',
        feerate: 30,
      },
      signOffer: {
        psbt: '',
        remote_peer: '',
        publish: false,
      },
      error: '',
    };
  },
  computed: {
    generatedPrepareKeysCommand() {
      return commandString(this.globalArgs, 'preparekeys', this.prepareKeys);
    },
    generatedMakeOfferCommand() {
      return commandString(this.globalArgs, 'makeoffer', this.makeOffer);
    },
    generatedSignOfferCommand() {
      return commandString(this.globalArgs, 'signoffer', this.signOffer);
    },
  },
  methods: {
    async downloadMatchFile() {
      const chanPointStr = this.matchFile.channels[0].chan_point.trim();
      if (!chanPointStr || !chanPointStr.includes(':')) {
        this.error = 'Invalid channel point format. Expected txid:outnum.';
        return
      }

      const key1 = this.matchFile.node1.identity_pubkey.trim();
      const key2 = this.matchFile.node2.identity_pubkey.trim();

      if (!key1 || !key2 || key1.length !== 66 || key2.length !== 66) {
        this.error = 'Both node public keys are required.';
        return;
      }

      try {
        let txid = chanPointStr.split(':')[0];
        let index = Number.parseInt(chanPointStr.split(':')[1], 10);
        const response = await fetch(`https://api.node-recovery.com/tx/${txid}`);
        if (!response.ok) {
          throw new Error('Error fetching transaction info from API: ' + response.statusText);
        }

        const json = await response.json();
        if (!json || !json.vout || !Array.isArray(json.vout) || json.vout.length < index) {
          throw new Error('Invalid transaction data received from API.');
        }

        const vOut = json.vout[index];
        this.matchFile.channels[0].address = vOut.scriptpubkey_address || '';
        this.matchFile.channels[0].capacity = vOut.value;

      } catch (error) {
        this.error = error.message;
        return;
      }

      const data = JSON.stringify(this.matchFile, null, 2);
      const blob = new Blob([data], {type: 'application/json'});
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `match-${key1.substring(0, 8)}-${key2.substring(0, 8)}.json`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
    },
  },
  mounted() {
    // To hide the main command output.
    this.$emit('generate-args', {hide: true});
  },
};
</script>

<style scoped>
.generated-command pre {
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
