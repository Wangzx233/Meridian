import { useLayoutEffect, useRef, useState } from "react";
import { clamp } from "./format";

export function useStoredPanelSize(key: string, fallback: number, min: number, max: number): [number, (value: number) => void] {
  const [size, setSizeState] = useState(() => {
    try {
      const raw = window.localStorage.getItem(key);
      const stored = raw ? Number(raw) : fallback;
      return clamp(Number.isFinite(stored) ? stored : fallback, min, max);
    } catch {
      return fallback;
    }
  });

  const setSize = (value: number) => {
    const next = clamp(value, min, max);
    setSizeState(next);
    try {
      window.localStorage.setItem(key, String(next));
    } catch {
      // Local storage can be unavailable in private or restricted browser modes.
    }
  };

  return [size, setSize];
}


function readStoredString(key: string | null, fallback: string): string {
  if (!key) {
    return fallback;
  }

  try {
    return window.localStorage.getItem(key) ?? fallback;
  } catch {
    return fallback;
  }
}

export function useStoredString(key: string | null, fallback: string): [string, (value: string) => void] {
  const keyRef = useRef<string | null>(key);
  const [stored, setStored] = useState(() => ({
    key,
    value: readStoredString(key, fallback),
  }));
  const value = stored.key === key ? stored.value : readStoredString(key, fallback);

  useLayoutEffect(() => {
    keyRef.current = key;
    if (stored.key === key && stored.value === value) {
      return;
    }
    setStored({ key, value });
  }, [key, stored.key, stored.value, value]);

  const setValue = (next: string) => {
    const activeKey = keyRef.current;
    setStored({
      key: activeKey,
      value: next,
    });
    try {
      if (!activeKey) {
        return;
      }
      if (next) {
        window.localStorage.setItem(activeKey, next);
      } else {
        window.localStorage.removeItem(activeKey);
      }
    } catch {
      // Local storage can be unavailable in private or restricted browser modes.
    }
  };

  return [value, setValue];
}
