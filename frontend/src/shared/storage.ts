import { useState } from "react";
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


export function useStoredString(key: string, fallback: string): [string, (value: string) => void] {
  const [value, setValueState] = useState(() => {
    try {
      return window.localStorage.getItem(key) ?? fallback;
    } catch {
      return fallback;
    }
  });

  const setValue = (next: string) => {
    setValueState(next);
    try {
      if (next) {
        window.localStorage.setItem(key, next);
      } else {
        window.localStorage.removeItem(key);
      }
    } catch {
      // Local storage can be unavailable in private or restricted browser modes.
    }
  };

  return [value, setValue];
}
