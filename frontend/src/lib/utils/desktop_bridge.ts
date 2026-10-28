import {SmartBridge} from '../../app/models/direct-smart';

export interface DesktopBridge {
  readonly smart?: SmartBridge;
  openExternal(url: string, state?: string): Promise<void>;
  closeAuthWindow(state: string): Promise<void>;
  completeAuth(data: Record<string, string>): Promise<void>;
  onAuthResponse(state: string, callback: (data: any) => void): () => void;
  getInfo(): Promise<{version: string}>;
}

declare global {
  interface Window { fastenDesktop?: DesktopBridge; }
}

// Inline runtime imports can finish after Angular starts. Wait for the explicit
// readiness event instead of depending on module download/execution ordering.
export function GetDesktopBridge(): Promise<DesktopBridge> {
  if (window.fastenDesktop) return Promise.resolve(window.fastenDesktop);
  return new Promise((resolve, reject) => {
    const ready = () => {
      if (window.fastenDesktop) { cleanup(); resolve(window.fastenDesktop); }
    };
    const timer = setTimeout(() => { cleanup(); reject(new Error('Desktop runtime did not start')); }, 10000);
    const cleanup = () => { clearTimeout(timer); window.removeEventListener('fasten:desktop-ready', ready); };
    window.addEventListener('fasten:desktop-ready', ready);
    ready();
  });
}
