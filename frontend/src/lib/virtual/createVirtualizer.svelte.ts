import { untrack } from "svelte";
import {
  Virtualizer,
  type VirtualizerOptions,
  observeElementOffset,
  observeElementRect,
  elementScroll,
  observeWindowOffset,
  observeWindowRect,
  windowScroll,
} from "@tanstack/virtual-core";

type PartialKeys<T, K extends keyof T> = Omit<T, K> &
  Partial<Pick<T, K>>;

// itemSizeCache is private in TanStack Virtual's types but
// we need it to transfer measurements across recreations.
type SizeCache = Map<string | number, number>;
// eslint-disable-next-line @typescript-eslint/no-explicit-any
function getSizeCache(v: any): SizeCache {
  return v?.itemSizeCache as SizeCache;
}
// eslint-disable-next-line @typescript-eslint/no-explicit-any
function setSizeCache(v: any, cache: SizeCache) {
  v.itemSizeCache = cache;
}

type ElementOpts = PartialKeys<
  VirtualizerOptions<HTMLElement, HTMLElement>,
  | "observeElementOffset"
  | "observeElementRect"
  | "scrollToFn"
> & { measureCacheKey?: unknown };

type WindowOpts = PartialKeys<
  VirtualizerOptions<Window, HTMLElement>,
  | "observeElementOffset"
  | "observeElementRect"
  | "scrollToFn"
  | "getScrollElement"
> & { measureCacheKey?: unknown };

export function createVirtualizer(
  optsFn: () => ElementOpts,
) {
  let instance: Virtualizer<HTMLElement, HTMLElement> =
    $state(undefined!);
  let notifyPending = false;
  let lastMeasureCacheKey: unknown = undefined;

  // TanStack Virtual calls onChange when measurements change,
  // passing the same virtualizer reference. Svelte 5's $state
  // uses referential equality, so `instance = vInst` (same
  // ref) is a no-op — no re-render. A version counter forces
  // re-evaluation when the getter is read.
  let _version = $state(0);

  function bumpVersion() {
    if (notifyPending) return;
    notifyPending = true;
    setTimeout(() => {
      notifyPending = false;
      _version++;
    }, 0);
  }

  $effect(() => {
    const opts = optsFn();

    // Save scroll position and measurement cache from the
    // previous virtualizer (read without tracking to avoid
    // circular dependencies).
    const prev = untrack(() => instance);
    const scrollEl = opts.getScrollElement?.() ?? null;
    const savedScrollTop = scrollEl?.scrollTop ?? 0;
    const prevCache = getSizeCache(prev);

    // If the cache key changes, discard the old measurements.
    // This prevents indefinite cache growth when keys are session-scoped.
    const shouldKeepCache =
      opts.measureCacheKey === lastMeasureCacheKey;
    lastMeasureCacheKey = opts.measureCacheKey;

    const savedCache: SizeCache =
      shouldKeepCache && prevCache?.size
        ? new Map(prevCache)
        : new Map();

    const resolvedOpts: VirtualizerOptions<
      HTMLElement,
      HTMLElement
    > = {
      observeElementOffset,
      observeElementRect,
      scrollToFn: elementScroll,
      ...opts,
      initialOffset: savedScrollTop,
      onChange: (
        vInst: Virtualizer<HTMLElement, HTMLElement>,
        sync: boolean,
      ) => {
        instance = vInst;
        if (sync) {
          bumpVersion();
        } else {
          _version++;
        }
        opts.onChange?.(vInst, sync);
      },
    };

    const v = new Virtualizer(resolvedOpts);

    // Transfer height measurements so the mapping from
    // scrollTop to item indices stays consistent.
    if (savedCache.size > 0) {
      setSizeCache(v, new Map(savedCache));
    }

    instance = v;
    v._willUpdate();

    // Restore scroll position after observer setup.
    // _willUpdate reads scrollTop from the DOM, but the
    // browser may have clamped it if the estimated total
    // height (from a fresh virtualizer) was smaller.
    if (scrollEl && savedScrollTop > 0) {
      scrollEl.scrollTop = savedScrollTop;
    }

    return () => {
      v._willUpdate();
    };
  });

  const activeInstance = $derived.by(() => {
    _version;
    return instance ? new Proxy(instance, {}) : instance;
  });

  return {
    get instance() {
      return activeInstance;
    },
  };
}

export function createWindowVirtualizer(
  optsFn: () => WindowOpts,
) {
  let instance: Virtualizer<Window, HTMLElement> =
    $state(undefined!);
  let notifyPending = false;
  let lastMeasureCacheKey: unknown = undefined;
  let _version = $state(0);

  function bumpVersion() {
    if (notifyPending) return;
    notifyPending = true;
    setTimeout(() => {
      notifyPending = false;
      _version++;
    }, 0);
  }

  $effect(() => {
    const opts = optsFn();

    const prev = untrack(() => instance);
    const savedOffset = prev?.scrollOffset ?? 0;
    const prevCache = getSizeCache(prev);

    const shouldKeepCache =
      opts.measureCacheKey === lastMeasureCacheKey;
    lastMeasureCacheKey = opts.measureCacheKey;

    const savedCache: SizeCache =
      shouldKeepCache && prevCache?.size
        ? new Map(prevCache)
        : new Map();

    const resolvedOpts: VirtualizerOptions<
      Window,
      HTMLElement
    > = {
      observeElementOffset: observeWindowOffset,
      observeElementRect: observeWindowRect,
      scrollToFn: windowScroll,
      getScrollElement: () => window,
      ...opts,
      initialOffset: savedOffset,
      onChange: (
        vInst: Virtualizer<Window, HTMLElement>,
        sync: boolean,
      ) => {
        instance = vInst;
        if (sync) {
          bumpVersion();
        } else {
          _version++;
        }
        opts.onChange?.(vInst, sync);
      },
    };

    const v = new Virtualizer(resolvedOpts);
    if (savedCache.size > 0) {
      setSizeCache(v, new Map(savedCache));
    }
    instance = v;
    v._willUpdate();

    return () => {
      v._willUpdate();
    };
  });

  const activeInstance = $derived.by(() => {
    _version;
    return instance ? new Proxy(instance, {}) : instance;
  });

  return {
    get instance() {
      return activeInstance;
    },
  };
}
