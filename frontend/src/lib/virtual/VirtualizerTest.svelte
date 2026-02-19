<script lang="ts">
  import { createVirtualizer, createWindowVirtualizer } from './createVirtualizer.svelte.js';

  interface Props {
    type: 'element' | 'window';
    options: any;
    onInstanceChange: (inst: any) => void;
  }

  let { type, options, onInstanceChange } = $props();

  const virtualizer = type === 'window'
    ? createWindowVirtualizer(() => options)
    : createVirtualizer(() => options);

  $effect(() => {
    onInstanceChange(virtualizer.instance);
  });

  export function getVirtualizer() {
    return virtualizer;
  }
</script>
