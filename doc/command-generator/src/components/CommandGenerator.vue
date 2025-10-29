<template>
  <div>
    <component
      :is="command.name"
      @generate-args="onGenerateArgs"
      :command="command"
      :global-args="globalArgs"
      v-if="command.name"
    />
    <div class="generated-command mt-4" v-if="!hideCommandOutput">
      <h3>Generated Command:</h3>
      <pre
        class="alert alert-secondary"
      ><code>{{ generatedCommand }}</code></pre>
    </div>
    <button @click="goBack" class="btn btn-secondary mt-3">Back</button>
  </div>
</template>

<script>
import SweepRemoteClosed from './SweepRemoteClosed.vue';
import TriggerForceClose from './TriggerForceClose.vue';
import ZombieRecovery from './ZombieRecovery.vue';

function commandString(comp) {
  let commandName = comp.command.name || '';
  let globalArgs = comp.globalArgs || {};
  let pageArgs = comp.pageArgs || {};

  if (!globalArgs.preCommand || !globalArgs.command) {
    return `chantools ${commandName.toLowerCase()}`;
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

  return `chantools ${globalArgsString}${commandName.toLowerCase()} ${commandArgs.trim()}`;
}

export default {
  name: 'CommandGenerator',
  components: {
    SweepRemoteClosed,
    TriggerForceClose,
    ZombieRecovery,
  },
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
      generatedCommand: commandString(this),
      pageArgs: {},
      hideCommandOutput: false,
    };
  },
  methods: {
    goBack() {
      this.$emit('back');
    },
    onGenerateArgs(args) {
      if (args.hide) {
        this.hideCommandOutput = true;
        this.pageArgs = {};
      } else {
        this.hideCommandOutput = false;
        this.pageArgs = args;
      }
      this.generatedCommand = commandString(this);
    },
  },
  watch: {
    command() {
      this.hideCommandOutput = false;
      this.generatedCommand = commandString(this);
    },
    globalArgs() {
      this.generatedCommand = commandString(this);
    },
  },
};
</script>

<style scoped>
.generated-command pre {
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
