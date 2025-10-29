<template>
  <div id="app" class="container mt-5">
    <h1 class="mb-4">chantools command generator</h1>
    <p class="lead">
      This tool helps you generate the correct command line arguments for
      various chantools commands based on your scenario.<br/>
      You must have <code>chantools</code> installed on your system to run the
      generated commands. See the
      <a href="https://github.com/lightninglabs/chantools#installation">installation instructions</a>.
      <br/><br/>
      Once you have installed <code>chantools</code>, start by specifying the
      global options below, then pick a command to run.
    </p>
    <GlobalOptions @global-args-changed="updateGlobalArgs" />
    <CommandSelector
        v-if="!selectedCommand"
        @command-selected="handleCommandSelected"
    />
    <CommandGenerator
        v-else
        :command="selectedCommand"
        :global-args="globalArgs"
        @back="goBack"
    />
  </div>
</template>

<script>
import CommandSelector from './components/CommandSelector.vue';
import CommandGenerator from './components/CommandGenerator.vue';
import GlobalOptions from './components/GlobalOptions.vue';

export default {
  name: 'App',
  components: {
    CommandSelector,
    CommandGenerator,
    GlobalOptions,
  },
  data() {
    return {
      selectedCommand: null,
      globalArgs: {
        preCommand: {},
        command: {},
      },
    };
  },
  methods: {
    handleCommandSelected(command) {
      this.selectedCommand = command;
    },
    goBack() {
      this.selectedCommand = null;
    },
    updateGlobalArgs(args) {
      this.globalArgs = args;
    },
  },
};
</script>

<style>
#app {
  font-family: Avenir, Helvetica, Arial, sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}
</style>
