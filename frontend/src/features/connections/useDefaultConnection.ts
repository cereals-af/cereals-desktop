import { useEffect, useRef } from 'react';
import type { Profile, State } from '../../api/types';

// A startup attempt is consumed once, including failure or cancellation. Refetching
// workspace state or manually disconnecting must never reconnect behind the user.
export function useDefaultConnection(
  state: State | undefined,
  connect: (profile: Profile, needsConfirmation: boolean) => void,
) {
  const attempted = useRef(false);
  useEffect(() => {
    if (!state || attempted.current) return;
    attempted.current = true;
    const profile = state.profiles?.find((item) => item.id === state.defaultClusterID);
    if (profile && !profile.connected) connect(profile, state.defaultNeedsConfirmation);
  }, [state, connect]);
}
