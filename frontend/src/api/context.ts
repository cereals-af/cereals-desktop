import { createContext, useContext } from 'react';
import type { DesktopAPI } from './types';

export const APIContext = createContext<DesktopAPI | null>(null);
export function useAPI(): DesktopAPI {
  const api = useContext(APIContext);
  if (!api) throw new Error('Desktop API provider is missing');
  return api;
}
