// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, unmount, tick } from 'svelte';
// @ts-ignore
import VirtualizerTest from './VirtualizerTest.svelte';

// Shared state to capture options passed to Virtualizer constructor
const { lastOptions, lastInstance } = vi.hoisted(() => ({ 
  lastOptions: { value: undefined as any },
  lastInstance: { value: undefined as any }
}));

vi.mock('@tanstack/virtual-core', async () => {
  const original = await vi.importActual<typeof import('@tanstack/virtual-core')>('@tanstack/virtual-core');
  return {
    ...original,
    Virtualizer: class {
      constructor(opts: any) {
        lastOptions.value = opts;
        lastInstance.value = this;
      }
      _willUpdate() {}
    },
    // Mock observer functions to prevent errors
    observeElementOffset: vi.fn(),
    observeElementRect: vi.fn(),
    elementScroll: vi.fn(),
    observeWindowOffset: vi.fn(),
    observeWindowRect: vi.fn(),
    windowScroll: vi.fn(),
  };
});

describe('createVirtualizer reactivity', () => {
  beforeEach(() => {
    lastOptions.value = undefined;
    lastInstance.value = undefined;
    vi.clearAllMocks();
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('updates when onChange fires with same reference (element virtualizer)', async () => {
    const onInstanceChange = vi.fn();
    const container = document.createElement('div');
    
    const component = mount(VirtualizerTest, {
      target: container,
      props: {
        type: 'element',
        options: { 
          count: 10, 
          getScrollElement: () => container,
          estimateSize: () => 50 
        },
        onInstanceChange
      }
    });

    await tick();

    // Initial render triggers one call
    expect(onInstanceChange).toHaveBeenCalledTimes(1);
    expect(lastOptions.value).toBeDefined();

    // Verify onChange wrapper exists
    const { onChange } = lastOptions.value;
    expect(typeof onChange).toBe('function');

    // Capture the current instance (proxy) from the callback
    const instanceProxy = onInstanceChange.mock.calls[0]![0];
    
    // Verify stability: multiple reads should return the same proxy reference
    // when no changes have occurred.
    expect(component.getVirtualizer().instance).toBe(instanceProxy);
    expect(component.getVirtualizer().instance).toBe(instanceProxy);
    
    // Get the raw instance from our mock capture
    const rawInstance = lastInstance.value;

    // Mutate raw instance to verify we are working with the same object underneath
    rawInstance._test_mutation = 'updated';
    
    // 1. Test sync update (onChange(..., false))
    // This should trigger _version++ (sync) and re-run the effect
    // We pass rawInstance as TanStack would
    onChange(rawInstance, false);
    await tick();
    // Ensure any timers are run just in case (though sync shouldn't need it)
    vi.advanceTimersByTime(100);
    await tick();

    expect(onInstanceChange).toHaveBeenCalledTimes(2);
    const receivedSync = onInstanceChange.mock.calls[1]![0];
    expect(receivedSync).not.toBe(instanceProxy);
    expect(receivedSync._test_mutation).toBe('updated');
    expect(component.getVirtualizer().instance).toBe(receivedSync);

    // 2. Test async update (onChange(..., true))
    // This should trigger bumpVersion() (setTimeout)
    onChange(rawInstance, true);
    
    // Should not have updated yet (setTimeout is pending)
    await tick();
    expect(onInstanceChange).toHaveBeenCalledTimes(2);

    // Advance timers to trigger the queued update
    vi.advanceTimersByTime(100);
    await tick();

    expect(onInstanceChange).toHaveBeenCalledTimes(3);
    const receivedAsync = onInstanceChange.mock.calls[2]![0];
    expect(receivedAsync).not.toBe(instanceProxy);
    expect(receivedAsync._test_mutation).toBe('updated');

    unmount(component);
  });

  it('updates when onChange fires with same reference (window virtualizer)', async () => {
    const onInstanceChange = vi.fn();
    const container = document.createElement('div');
    
    const component = mount(VirtualizerTest, {
      target: container,
      props: {
        type: 'window',
        options: { 
          count: 20, 
          estimateSize: () => 50 
        },
        onInstanceChange
      }
    });

    await tick();

    expect(onInstanceChange).toHaveBeenCalledTimes(1);
    expect(lastOptions.value).toBeDefined();

    const { onChange } = lastOptions.value;
    const instanceProxy = onInstanceChange.mock.calls[0]![0];
    
    // Verify stability: multiple reads should return the same proxy reference
    expect(component.getVirtualizer().instance).toBe(instanceProxy);
    expect(component.getVirtualizer().instance).toBe(instanceProxy);

    const rawInstance = lastInstance.value;

    // 1. Test sync update
    onChange(rawInstance, false);
    await tick();
    vi.advanceTimersByTime(100);
    await tick();

    expect(onInstanceChange).toHaveBeenCalledTimes(2);
    expect(onInstanceChange.mock.calls[1]![0]).not.toBe(instanceProxy);
    expect(component.getVirtualizer().instance).toBe(onInstanceChange.mock.calls[1]![0]);

    // 2. Test async update
    onChange(rawInstance, true);
    await tick();
    expect(onInstanceChange).toHaveBeenCalledTimes(2);

    vi.advanceTimersByTime(100);
    await tick();

    expect(onInstanceChange).toHaveBeenCalledTimes(3);
    expect(onInstanceChange.mock.calls[2]![0]).not.toBe(instanceProxy);

    unmount(component);
  });
});
