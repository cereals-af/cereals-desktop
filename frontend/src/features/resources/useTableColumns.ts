import { useState } from 'react';

interface Preference {
  hidden: string[];
  unavailable: boolean;
}

function readPreference(key: string): Preference {
  try {
    const value: unknown = JSON.parse(localStorage.getItem(key) ?? '[]');
    return {
      hidden: Array.isArray(value)
        ? value.filter((item): item is string => typeof item === 'string')
        : [],
      unavailable: false,
    };
  } catch {
    return { hidden: [], unavailable: true };
  }
}

export function useTableColumns(scope: string) {
  const key = `creals.table-columns.v1.${scope}`;
  const [preference, setPreference] = useState(() => readPreference(key));
  const toggle = (column: string, checked: boolean) => {
    const hidden = checked
      ? preference.hidden.filter((item) => item !== column)
      : [...preference.hidden, column];
    let unavailable = false;
    try {
      localStorage.setItem(key, JSON.stringify(hidden));
    } catch {
      unavailable = true;
    }
    setPreference({ hidden, unavailable });
  };
  return { ...preference, toggle, shown: (column: string) => !preference.hidden.includes(column) };
}
