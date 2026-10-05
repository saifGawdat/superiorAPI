"use client";

import { useCallback, useEffect, useRef, useState, useSyncExternalStore } from "react";

const REDUCED_MOTION = "(prefers-reduced-motion: reduce)";

function subscribeReducedMotion(onChange: () => void) {
  const mq = window.matchMedia(REDUCED_MOTION);
  mq.addEventListener("change", onChange);
  return () => mq.removeEventListener("change", onChange);
}

export function useReducedMotion(): boolean {
  return useSyncExternalStore(
    subscribeReducedMotion,
    () => window.matchMedia(REDUCED_MOTION).matches,
    () => false,
  );
}

/**
 * Tweens toward `target` over `duration` ms. Returns the target directly when
 * the user prefers reduced motion or there is nothing to animate from.
 */
export function useAnimatedNumber(target: number | null, duration = 450): number | null {
  const reduced = useReducedMotion();
  const [shown, setShown] = useState<number | null>(target);
  const current = useRef<number | null>(target);

  useEffect(() => {
    if (reduced || target == null) return;
    const from = current.current ?? target;
    let frame = 0;
    let start: number | null = null;
    const step = (now: number) => {
      if (start == null) start = now;
      const t = Math.min(1, (now - start) / duration);
      const value = from + (target - from) * (1 - (1 - t) ** 3);
      current.current = value;
      setShown(value);
      if (t < 1) frame = requestAnimationFrame(step);
    };
    frame = requestAnimationFrame(step);
    // Frames are throttled in background tabs; never leave a stale number.
    const snap = setTimeout(() => {
      cancelAnimationFrame(frame);
      current.current = target;
      setShown(target);
    }, duration + 120);
    return () => {
      cancelAnimationFrame(frame);
      clearTimeout(snap);
    };
  }, [target, duration, reduced]);

  if (reduced || target == null || shown == null) return target;
  return shown;
}

/** Measures an element's content width with ResizeObserver. */
export function useElementWidth<T extends HTMLElement>(): [
  (node: T | null) => void,
  number,
] {
  const [node, setNode] = useState<T | null>(null);
  const [width, setWidth] = useState(0);
  const ref = useCallback((el: T | null) => setNode(el), []);

  useEffect(() => {
    if (!node) return;
    const observer = new ResizeObserver((entries) => {
      const w = entries[0]?.contentRect.width ?? 0;
      setWidth(Math.round(w));
    });
    observer.observe(node);
    return () => observer.disconnect();
  }, [node]);

  return [ref, width];
}

/** Re-renders at a fixed interval while `active`; returns performance.now(). */
export function useTicker(active: boolean, intervalMs = 100): number {
  const [now, setNow] = useState(0);
  useEffect(() => {
    if (!active) return;
    const id = setInterval(() => setNow(performance.now()), intervalMs);
    return () => clearInterval(id);
  }, [active, intervalMs]);
  return now;
}
