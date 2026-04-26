import * as App from '../../wailsjs/go/main/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';

export { App };

/** Subscribe to a Wails event. Returns an unsubscribe function. */
export function on<T = unknown>(event: string, handler: (payload: T) => void): () => void {
  return EventsOn(event, (...data: unknown[]) => {
    handler(data[0] as T);
  });
}
