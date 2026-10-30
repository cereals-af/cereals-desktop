import { useState } from 'react';
import type { Ref } from '../../api/types';
export interface PodTarget {
  target: Ref;
  uid: string;
}
export function usePodConsoles(initial?: PodTarget) {
  const [entries, setEntries] = useState<PodTarget[]>(initial ? [initial] : []);
  const [active, setActive] = useState(initial?.uid ?? '');
  const open = (pod: PodTarget) => {
    if (!entries.some((entry) => entry.uid === pod.uid) && entries.length >= 8) return;
    setEntries((items) => (items.some((entry) => entry.uid === pod.uid) ? items : [...items, pod]));
    setActive(pod.uid);
  };
  const close = (uid: string) => {
    setEntries((items) => items.filter((entry) => entry.uid !== uid));
    if (active === uid) setActive(entries.find((entry) => entry.uid !== uid)?.uid ?? '');
  };
  return { entries, active, setActive, open, close };
}
