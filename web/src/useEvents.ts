import { useEffect, useReducer, useRef } from "react";
import { withToken, type RconEvent } from "./api";

/** Maximum events kept in memory per session.
 *
 * A long-running console would otherwise grow without bound. The list is
 * virtualised for rendering, but the array itself still has to be capped.
 */
const MAX_EVENTS = 5000;

type Action =
  | { type: "append"; event: RconEvent }
  | { type: "reset" };

function reducer(state: RconEvent[], action: Action): RconEvent[] {
  switch (action.type) {
    case "reset":
      return [];

    case "append": {
      const e = action.event;

      // The stream is ordered by Seq, but a reconnect can replay events the
      // client already holds. Dropping anything at or below the newest Seq
      // makes resumption idempotent rather than duplicating the console.
      const last = state[state.length - 1];
      if (last && e.seq <= last.seq) return state;

      const next = state.length >= MAX_EVENTS ? state.slice(state.length - MAX_EVENTS + 1) : state;
      return [...next, e];
    }
  }
}

/** useEvents subscribes to the daemon's event stream for one server.
 *
 * EventSource handles reconnection and resends Last-Event-ID on its own, which
 * is most of why the transport is SSE. The `lastSeq` ref exists only for the
 * initial connect, so a remount resumes rather than replaying from the start.
 */
export function useEvents(profileId: string | null) {
  const [events, dispatch] = useReducer(reducer, []);
  const lastSeq = useRef(0);

  useEffect(() => {
    dispatch({ type: "reset" });
    lastSeq.current = 0;

    if (!profileId) return;

    const url = withToken(
      `/api/events?profileId=${encodeURIComponent(profileId)}&lastEventId=${lastSeq.current}`,
    );
    const source = new EventSource(url);

    const onMessage = (raw: MessageEvent) => {
      try {
        const e: RconEvent = JSON.parse(raw.data);
        lastSeq.current = Math.max(lastSeq.current, e.seq);
        dispatch({ type: "append", event: e });
      } catch {
        // A malformed frame should not tear down the stream.
      }
    };

    // The daemon sets a per-stream `event:` name, so the default "message"
    // handler never fires; each named type has to be registered.
    for (const name of ["command", "response", "status", "error"]) {
      source.addEventListener(name, onMessage);
    }

    return () => source.close();
  }, [profileId]);

  return events;
}
