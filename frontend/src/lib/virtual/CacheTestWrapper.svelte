<script lang="ts">
  import VirtualizerTest from './VirtualizerTest.svelte';

  type Options = { measureCacheKey: string; count: number };

  interface Props {
    type: 'element' | 'window';
    controller: {
        initialOptions: Options;
        updateOptions: (opts: Options) => void;
    };
    onInstanceChange: (inst: unknown) => void;
  }

  let { type, controller, onInstanceChange }: Props = $props();
  
  let options = $state(controller.initialOptions);

  controller.updateOptions = (newOpts: Options) => {
    options = newOpts;
  };
</script>

<VirtualizerTest {type} {options} {onInstanceChange} />
