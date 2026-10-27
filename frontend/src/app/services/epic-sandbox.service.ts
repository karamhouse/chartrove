import {Injectable} from '@angular/core';
import {HttpBackend, HttpClient, HttpHeaders} from '@angular/common/http';
import {Observable, fromEvent} from 'rxjs';
import {takeUntil, timeout} from 'rxjs/operators';
import {Patient} from 'fhir/r4';
import {DesktopBridge, GetDesktopBridge} from '../../lib/utils/desktop_bridge';
import {uuidV4} from '../../lib/utils/uuid';

export const EPIC_SANDBOX = {
  metadata: 'https://lighthouse.fastenhealth.com/sandbox/connect/8e2f5de7-46ac-4067-96ba-5e3f60ad52a4',
  endpoint: '8e2f5de7-46ac-4067-96ba-5e3f60ad52a4',
  brand: 'e16b9952-8885-4905-b2e3-b0f04746ed5c',
  portal: '2727ec27-67e9-475a-bea1-423102beaa1d',
  patient: 'erXuFYUfucBZaryVksYEcMg3',
  fhir: 'https://fhir.epic.com/interconnect-fhir-oauth/api/FHIR/R4/',
  authorize: 'https://fhir.epic.com/interconnect-fhir-oauth/oauth2/authorize',
  token: 'https://fhir.epic.com/interconnect-fhir-oauth/oauth2/token',
  callback: 'https://lighthouse.fastenhealth.com/sandbox/callback/epic'
};

@Injectable({providedIn: 'root'})
export class EpicSandboxService {
  private http: HttpClient;

  constructor(backend: HttpBackend) {
    // External OAuth requests must not inherit the local app's JWT or request
    // logging/auth-error interceptors. Only the later local import uses those.
    this.http = new HttpClient(backend);
  }

  async connect(signal: AbortSignal, progress: (message: string) => void): Promise<Patient> {
    this.checkCancelled(signal);
    progress('Loading Epic sandbox…');
    const wrapper = await this.request(this.http.get<any>(EPIC_SANDBOX.metadata), signal);
    const metadata = wrapper?.data;
    if (!wrapper?.success || !metadata || metadata.id !== EPIC_SANDBOX.endpoint ||
      metadata.authorization_endpoint !== EPIC_SANDBOX.authorize || metadata.token_endpoint !== EPIC_SANDBOX.token ||
      metadata.url !== EPIC_SANDBOX.fhir || metadata.redirect_uri !== EPIC_SANDBOX.callback ||
      metadata.confidential !== false || !/^[a-zA-Z0-9-]{1,128}$/.test(metadata.client_id || '')) {
      throw new Error('Epic sandbox metadata is unavailable or has changed.');
    }
    const bridge = await GetDesktopBridge();
    this.checkCancelled(signal);
    const state = uuidV4();
    const encode = (bytes: Uint8Array) => btoa(String.fromCharCode(...bytes)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
    const verifier = encode(crypto.getRandomValues(new Uint8Array(32)));
    const challenge = encode(new Uint8Array(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(verifier))));
    this.checkCancelled(signal);
    const authorize = new URL(EPIC_SANDBOX.authorize);
    for (const [key, value] of Object.entries({
      client_id: metadata.client_id, redirect_uri: EPIC_SANDBOX.callback, response_type: 'code', response_mode: 'query',
      scope: 'openid fhirUser profile', state, aud: EPIC_SANDBOX.fhir, code_challenge: challenge, code_challenge_method: 'S256'
    })) authorize.searchParams.set(key, value);
    const redirect = new URL('https://lighthouse.fastenhealth.com/sandbox/redirect/' + state);
    for (const [key, value] of Object.entries({
      origin_url: window.location.origin + '/#desktop/callback/' + state,
      dest_url: authorize.toString(), desktop_mode: 'true', brand_id: EPIC_SANDBOX.brand,
      portal_id: EPIC_SANDBOX.portal, endpoint_id: EPIC_SANDBOX.endpoint
    })) redirect.searchParams.set(key, value);

    try {
      progress('Sign in with Epic’s published synthetic test account in the popup.');
      const data = await this.waitForCallback(bridge, state, redirect.toString(), signal);
      this.checkCancelled(signal);
      if (data.state !== state) throw new Error('The provider callback state did not match this connection.');
      if (data.error || typeof data.code !== 'string' || !data.code) throw new Error('Epic sandbox access was denied or cancelled.');
      progress('Retrieving the synthetic Patient…');
      const body = new URLSearchParams({grant_type: 'authorization_code', client_id: metadata.client_id,
        code: data.code, redirect_uri: EPIC_SANDBOX.callback, code_verifier: verifier});
      const tokens = await this.request(this.http.post<any>(EPIC_SANDBOX.token, body.toString(), {
        headers: new HttpHeaders({'Content-Type': 'application/x-www-form-urlencoded'})
      }), signal);
      if (!tokens?.access_token || typeof tokens.access_token !== 'string' ||
        tokens.token_type?.toLowerCase() !== 'bearer' || tokens.patient !== EPIC_SANDBOX.patient) {
        throw new Error('The sandbox did not return the expected synthetic patient authorization.');
      }
      try {
        const patient = await this.request(this.http.get<Patient>(EPIC_SANDBOX.fhir + 'Patient/' + EPIC_SANDBOX.patient, {
          headers: new HttpHeaders({Authorization: 'Bearer ' + tokens.access_token, Accept: 'application/fhir+json'})
        }), signal);
        if (patient?.resourceType !== 'Patient' || patient.id !== EPIC_SANDBOX.patient) {
          throw new Error('The provider response was not the expected synthetic Patient.');
        }
        return patient;
      } finally {
        delete tokens.access_token;
        delete tokens.refresh_token;
        delete tokens.id_token;
      }
    } finally {
      await bridge.closeAuthWindow(state).catch(() => {});
    }
  }

  private async request<T>(request: Observable<T>, signal: AbortSignal): Promise<T> {
    this.checkCancelled(signal);
    try {
      const result = await request.pipe(timeout(30000), takeUntil(fromEvent(signal, 'abort'))).toPromise();
      this.checkCancelled(signal);
      return result;
    } catch (error) {
      this.checkCancelled(signal);
      // Never display or log server response bodies, codes, or token payloads.
      throw new Error('The Epic sandbox request failed. Please try again later.');
    }
  }

  private checkCancelled(signal: AbortSignal) {
    if (signal.aborted) throw new Error('Connection cancelled.');
  }

  private waitForCallback(bridge: DesktopBridge, state: string, url: string, signal: AbortSignal): Promise<any> {
    return new Promise((resolve, reject) => {
      let stop = () => {};
      let settled = false;
      const finish = (error?: Error, data?: any) => {
        if (settled) return;
        settled = true;
        clearTimeout(timer);
        stop();
        signal.removeEventListener('abort', cancel);
        error ? reject(error) : resolve(data);
      };
      const cancel = () => finish(new Error('Connection cancelled.'));
      const timer = setTimeout(() => finish(new Error('Epic sandbox sign-in timed out. Please try again.')), 120000);
      signal.addEventListener('abort', cancel, {once: true});
      stop = bridge.onAuthResponse(state, data => finish(undefined, data));
      if (signal.aborted) cancel();
      if (!settled) bridge.openExternal(url, state).catch(() => finish(new Error('Could not open the Epic sandbox sign-in window.')));
    });
  }
}
