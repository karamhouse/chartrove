import {Inject, Injectable, InjectionToken} from '@angular/core';
import {Patient} from 'fhir/r4';
import {DesktopBridge, GetDesktopBridge} from '../../lib/utils/desktop_bridge';
import {SmartBridge, SmartConnection, SmartProvider} from '../models/direct-smart';

function isRecord(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === 'object' && !Array.isArray(value);
}

export class DirectSmartCleanupError extends Error {
  constructor() { super('Could not release the direct session. Close Chartrove to clear it.'); }
}

export const SMART_DESKTOP_BRIDGE = new InjectionToken<() => Promise<Pick<DesktopBridge, 'smart'>>>(
  'SMART_DESKTOP_BRIDGE', {providedIn: 'root', factory: () => GetDesktopBridge});

@Injectable({providedIn: 'root'})
export class DirectSmartService {
  constructor(@Inject(SMART_DESKTOP_BRIDGE) private getDesktopBridge: () => Promise<Pick<DesktopBridge, 'smart'>>) {}

  private async native(): Promise<SmartBridge> {
    try {
      const {smart} = await this.getDesktopBridge();
      if (!smart) throw new Error();
      return smart;
    } catch { throw new Error('Direct connections require the current Chartrove desktop host.'); }
  }

  async providers(): Promise<SmartProvider[]> {
    try {
      const providers = await (await this.native()).providers();
      if (!Array.isArray(providers)) throw new Error();
      return providers.map(provider => {
        if (!provider || typeof provider.id !== 'string' || !/^[a-z0-9][a-z0-9-]{0,63}$/.test(provider.id) ||
          typeof provider.display !== 'string' || !provider.display || provider.display.length > 100 ||
          typeof provider.fhir_base_url !== 'string' || new URL(provider.fhir_base_url).protocol !== 'https:') throw new Error();
        return {id: provider.id, display: provider.display, fhir_base_url: provider.fhir_base_url};
      });
    } catch {
      throw new Error('Could not load direct providers. Check this installation’s configuration and try again.');
    }
  }

  async connect(providerID: string, signal: AbortSignal): Promise<{connection: SmartConnection, patient: Patient}> {
    const smart = await this.native();
    let connectionID: string | undefined;
    let keep = false;
    const cleanup: Promise<PromiseSettledResult<void>[]>[] = [];
    const abort = () => {
      // Teardown must handle both promises, even if the native app is quitting.
      cleanup.push(Promise.allSettled([smart.cancel(providerID), ...(connectionID ? [smart.disconnect(connectionID)] : [])]));
    };
    signal.addEventListener('abort', abort, {once: true});
    try {
      if (signal.aborted) throw new Error();
      const received = await smart.connect(providerID);
      connectionID = typeof received?.id === 'string' ? received.id : undefined;
      if (signal.aborted || !connectionID || received.provider_id !== providerID ||
        typeof received.patient_id !== 'string' || !/^[A-Za-z0-9.-]{1,64}$/.test(received.patient_id) ||
        !Number.isFinite(received.expires_at) || received.expires_at <= Date.now() / 1000) throw new Error();
      const connection: SmartConnection = {id: connectionID, provider_id: providerID,
        patient_id: received.patient_id, expires_at: received.expires_at};
      const parsed: unknown = JSON.parse(await smart.readPatient(connectionID));
      if (signal.aborted || !isRecord(parsed) || parsed['resourceType'] !== 'Patient' || parsed['id'] !== connection.patient_id) throw new Error();
      // The Go host verifies the same identity. Only the FHIR Patient crosses
      // this boundary; no provider credentials or arbitrary request URLs do.
      const patient = parsed as unknown as Patient;
      keep = true;
      return {connection, patient};
    } catch {
      throw new Error(signal.aborted ? 'Connection cancelled.' : 'The direct connection failed. Reconnect to try again.');
    } finally {
      signal.removeEventListener('abort', abort);
      if (!keep && connectionID) cleanup.push(Promise.allSettled([smart.disconnect(connectionID)]));
      const results = await Promise.all(cleanup);
      if (results.some(group => group.some(result => result.status === 'rejected'))) {
        throw new DirectSmartCleanupError();
      }
    }
  }

  async disconnect(connectionID: string): Promise<void> {
    try { await (await this.native()).disconnect(connectionID); }
    catch { throw new DirectSmartCleanupError(); }
  }
}
