const service = 'github.com/karamhouse/fasten-desktop/internal/shell.DesktopService';
const smartService = 'github.com/karamhouse/fasten-desktop/internal/shell.SMARTService';

// The host serves the matching runtime. Do not bundle a second runtime version.
export function createDesktopBridge(runtime) {
  return Object.freeze({
    smart: Object.freeze({
      providers: () => runtime.Call.ByName(`${smartService}.Providers`),
      connect: providerID => runtime.Call.ByName(`${smartService}.Connect`, providerID),
      readPatient: connectionID => runtime.Call.ByName(`${smartService}.ReadPatient`, connectionID),
      cancel: providerID => runtime.Call.ByName(`${smartService}.Cancel`, providerID),
      disconnect: connectionID => runtime.Call.ByName(`${smartService}.Disconnect`, connectionID),
    }),
    openExternal: (url, state = 'external') => runtime.Call.ByName(`${service}.OpenExternal`, url, state),
    closeAuthWindow: state => runtime.Call.ByName(`${service}.CloseAuthWindow`, state),
    completeAuth: data => runtime.Call.ByName(`${service}.CompleteAuth`, data),
    getInfo: () => runtime.Call.ByName(`${service}.Info`),
    onAuthResponse(state, callback) {
      let active = true;
      let remove = () => {};
      const stop = () => {
        if (!active) return;
        active = false;
        remove();
      };
      remove = runtime.Events.On('fasten:source-auth-response', event => {
        if (!active || event.data?.state !== state) return;
        stop();
        callback(event.data);
      });
      return stop;
    },
  });
}
