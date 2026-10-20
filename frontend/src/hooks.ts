import { useEffect, useRef, useState, type RefObject } from "react";

export function useSize<T extends HTMLElement>(): [RefObject<T | null>, { width: number; height: number }] {
  const ref = useRef<T | null>(null);
  const [size, setSize] = useState({ width: 0, height: 0 });
  useEffect(() => {
    const element = ref.current;
    if (!element) {
      return;
    }
    const observer = new ResizeObserver(([entry]) => {
      if (entry) {
        setSize({ width: entry.contentRect.width, height: entry.contentRect.height });
      }
    });
    observer.observe(element);
    return () => observer.disconnect();
  }, []);
  return [ref, size];
}

export function useDebounced<T>(callback: (value: T) => void, delay: number): (value: T) => void {
  const timer = useRef<number | undefined>(undefined);
  const latest = useRef(callback);
  latest.current = callback;
  useEffect(() => () => window.clearTimeout(timer.current), []);
  return (value: T) => {
    window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => latest.current(value), delay);
  };
}
