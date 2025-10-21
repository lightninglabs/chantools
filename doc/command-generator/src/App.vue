<template>
  <div id="app" class="container mt-5">
    <h1 class="mb-4">chantools command generator</h1>
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
