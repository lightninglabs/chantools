<template>
  <div class="card mb-4">
    <div class="card-body">
      <h2 class="card-title">Global Options</h2>
      <div class="row form-group">
        <div class="col-md-6 mb-3">
          <label for="seed_source" class="form-label">Seed source</label>
          <select
              class="form-select"
              id="seed_source"
              v-model="args.seedSource"
              @input="updateGlobalArgs"
              @keyup="updateGlobalArgs"
          >
            <option value="terminal">
              Enter seed manually in terminal when asked by chantools
            </option>
            <option value="cln">Derive from CLN hsm_secret</option>
            <option value="rootkey">Use extended root key (xpriv)</option>
            <option value="walletdb">Extract from LND wallet database</option>
          </select>
          <div class="form-text">
            How you want to provide your LN node secret seed phrase to
            <code>chantools</code> (which is required for almost all commands).
          </div>
        </div>
        <div class="col-md-6 mb-3" v-if="args.seedSource === 'rootkey'">
          <label for="rootkey" class="form-label">
            Extended root key (xpriv)
          </label>
          <input
              type="text"
              class="form-control"
              id="rootkey"
              v-model="args.rootkey"
              @input="updateGlobalArgs"
              @keyup="updateGlobalArgs"
          />
          <div class="form-text">
            The BIP32 HD extended root private key (xpriv) to use for deriving
            the multisig keys; must be in standard Base58 format starting with
            <code>xprv</code>, <code>tprv</code>, <code>yprv</code> or
            <code>zprv</code>.
          </div>
        </div>
        <div class="col-md-6 mb-3" v-if="args.seedSource === 'cln'">
          <label for="hsm_secret" class="form-label">HSM secret</label>
          <input
              type="text"
              class="form-control"
              id="hsm_secret"
              v-model="args.hsm_secret"
              @input="updateGlobalArgs"
              @keyup="updateGlobalArgs"
          />
          <div class="form-text">
            The hex encoded HSM secret to use for deriving the multisig keys for
            a CLN node; obtain by running <code>xxd -p -c32
            ~/.lightning/bitcoin/hsm_secret</code> on the machine you're running
            CLN on.
          </div>
        </div>
        <div class="col-md-6 mb-3" v-if="args.seedSource === 'walletdb'">
          <label for="walletdb" class="form-label">Wallet database file</label>
          <input
              type="text"
              class="form-control"
              id="walletdb"
              v-model="args.walletdb"
              @input="updateGlobalArgs"
              @keyup="updateGlobalArgs"
          />
          <div class="form-text">
            The full path to the LND wallet database file (wallet.db); typically
            located at <code>~/.lnd/data/chain/bitcoin/mainnet/wallet.db</code>.
            Currently only supports <code>bbolt</code>! SQLite databases are not
            yet supported.
          </div>
        </div>
      </div>
      <div class="row">
        <div class="col-md-6 mb-3">
          <label for="apiurl" class="form-label">API URL</label>
          <input
              type="text"
              class="form-control"
              id="apiurl"
              v-model="args.apiurl"
              @input="updateGlobalArgs"
              @keyup="updateGlobalArgs"
              placeholder="https://api.node-recovery.com"
          />
          <div class="form-text">
            URL to use for address lookups (must be <code>esplora</code>
            compatible). Use your own if available for better privacy.
          </div>
        </div>
        <div class="col-md-4 mb-3 form-group">
          <label for="network">Bitcoin network</label>
          <select
              class="form-select"
              id="network"
              v-model="args.network"
              @change="updateGlobalArgs"
          >
            <option value="mainnet">mainnet</option>
            <option value="testnet">testnet</option>
            <option value="regtest">regtest</option>
            <option value="signet">signet</option>
          </select>
          <div class="form-text">
            What Bitcoin network to use.
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
const defaultValues = {
  apiurl: 'https://api.node-recovery.com',
  seedSource: 'terminal',
  network: 'mainnet',
}

export default {
  name: 'GlobalOptions',
  emits: ['global-args-changed'],
  data() {
    return {
      args: {
        apiurl: defaultValues.apiurl,
        seedSource: defaultValues.seedSource,
        hsm_secret: '',
        rootkey: '',
        walletdb: '',
        network: defaultValues.network,
      },
    };
  },
  methods: {
    updateGlobalArgs() {
      let args = {
        preCommand: {},
        command: {},
      };
      for (const [key, value] of Object.entries(this.args)) {
        // The dropdown is converted into single booleans.
        if (key === 'network' && value !== defaultValues.network) {
          args.preCommand[value] = true;
          continue;
        }
        
        // The seed source is a "virtual" argument that translates into other
        // args.
        if (key === 'seedSource' && value !== defaultValues.seedSource) {
          continue;
        }
        
        if (value && value !== defaultValues[key]) {
          args.command[key] = value;
        }
      }
      this.$emit('global-args-changed', args);
    },
  },
};
</script>
