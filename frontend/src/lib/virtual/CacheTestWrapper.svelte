<script lang="ts">
  import VirtualizerTest from './VirtualizerTest.svelte';

  interface Props {
    type: 'element' | 'window';
    controller: { 
        initialOptions: any; 
        updateOptions?: (opts: any) => void 
    };
    onInstanceChange: (inst: any) => void;
  }

  let { type, controller, onInstanceChange }: Props = $props();
  
  let options = $state(controller.initialOptions);

  controller.updateOptions = (newOpts: any) => {
    options = newOpts;
  };
</script>

<VirtualizerTest {type} {options} {onInstanceChange} />
