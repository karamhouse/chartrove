import {TestBed} from '@angular/core/testing';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import {EpicSandboxService, EPIC_SANDBOX} from './epic-sandbox.service';
import { provideHttpClient, withInterceptorsFromDi } from '@angular/common/http';

describe('Epic sandbox connection', () => {
  let service: EpicSandboxService;
  let http: HttpTestingController;
  let callback: (data: any) => void;
  let opened: Promise<string>;
  let open: (url: string) => void;
  let stopped: boolean;
  let closed: boolean;
  const metadata = {
    id: '8e2f5de7-46ac-4067-96ba-5e3f60ad52a4',
    authorization_endpoint: 'https://fhir.epic.com/interconnect-fhir-oauth/oauth2/authorize',
    token_endpoint: 'https://fhir.epic.com/interconnect-fhir-oauth/oauth2/token',
    url: 'https://fhir.epic.com/interconnect-fhir-oauth/api/FHIR/R4/',
    redirect_uri: 'https://lighthouse.fastenhealth.com/sandbox/callback/epic',
    client_id: 'a2436bd1-238f-4f74-8e5a-af0dbd6ff1df', confidential: false
  };

  beforeEach(() => {
    TestBed.configureTestingModule({ imports: [], providers: [provideHttpClient(withInterceptorsFromDi()), provideHttpClientTesting()] });
    service = TestBed.inject(EpicSandboxService);
    http = TestBed.inject(HttpTestingController);
    stopped = closed = false;
    opened = new Promise(resolve => open = resolve);
    window.fastenDesktop = {
      getInfo: async () => ({version: 'test'}),
      openExternal: async url => { open(url); },
      onAuthResponse: (_state, receive) => { callback = receive; return () => stopped = true; },
      closeAuthWindow: async () => { closed = true; },
      completeAuth: async () => {}
    };
  });
  afterEach(() => { http.verify(); delete window.fastenDesktop; });

  async function begin(signal = new AbortController().signal) {
    const result = service.connect(signal, () => {});
    http.expectOne(EPIC_SANDBOX.metadata).flush({success: true, data: metadata});
    const redirect = new URL(await opened);
    const auth = new URL(redirect.searchParams.get('dest_url'));
    return {result, redirect, auth};
  }

  it('exchanges a state-bound PKCE code and returns only the synthetic Patient', async () => {
    const {result, redirect, auth} = await begin();
    expect(redirect.origin).toBe('https://lighthouse.fastenhealth.com');
    expect(redirect.pathname).toContain('/sandbox/redirect/');
    expect(auth.searchParams.get('code_challenge_method')).toBe('S256');
    callback({state: auth.searchParams.get('state'), code: 'synthetic-code'});
    await new Promise(resolve => setTimeout(resolve, 0));
    const exchange = http.expectOne(metadata.token_endpoint);
    const body = new URLSearchParams(exchange.request.body);
    expect(body.get('code')).toBe('synthetic-code');
    expect(body.get('client_id')).toBe(metadata.client_id);
    expect(body.get('redirect_uri')).toBe(metadata.redirect_uri);
    expect(body.get('code_verifier').length).toBeGreaterThanOrEqual(43);
    expect(exchange.request.headers.has('Authorization')).toBeFalse();
    exchange.flush({access_token: 'synthetic-bearer', token_type: 'Bearer', patient: EPIC_SANDBOX.patient});
    await new Promise(resolve => setTimeout(resolve, 0));
    const patient = http.expectOne(metadata.url + 'Patient/' + EPIC_SANDBOX.patient);
    expect(patient.request.headers.get('Authorization')).toBe('Bearer synthetic-bearer');
    patient.flush({resourceType: 'Patient', id: EPIC_SANDBOX.patient});
    expect(await result).toEqual({resourceType: 'Patient', id: EPIC_SANDBOX.patient});
    expect(stopped).toBeTrue();
    expect(closed).toBeTrue();
  });

  it('rejects mismatched callback state before token exchange', async () => {
    const {result} = await begin();
    callback({state: 'different-state', code: 'synthetic-code'});
    await expectAsync(result).toBeRejectedWithError(/state/);
    http.expectNone(metadata.token_endpoint);
  });

  it('cancels the popup and detaches its listener', async () => {
    const controller = new AbortController();
    const {result} = await begin(controller.signal);
    controller.abort();
    await expectAsync(result).toBeRejectedWithError(/cancelled/);
    expect(stopped).toBeTrue();
    expect(closed).toBeTrue();
    http.expectNone(metadata.token_endpoint);
  });

  it('refuses metadata that would send tokens to a different host', async () => {
    const result = service.connect(new AbortController().signal, () => {});
    http.expectOne(EPIC_SANDBOX.metadata).flush({success: true, data: {...metadata, token_endpoint: 'https://other.example/token'}});
    await expectAsync(result).toBeRejectedWithError(/metadata/);
  });

  it('rejects provider denial without attempting token exchange', async () => {
    const {result, auth} = await begin();
    callback({state: auth.searchParams.get('state'), error: 'access_denied'});
    await expectAsync(result).toBeRejectedWithError(/denied/);
    http.expectNone(metadata.token_endpoint);
  });

  it('rejects unexpected patient context before retrieving any record', async () => {
    const {result, auth} = await begin();
    callback({state: auth.searchParams.get('state'), code: 'synthetic-code'});
    await new Promise(resolve => setTimeout(resolve, 0));
    http.expectOne(metadata.token_endpoint).flush({access_token: 'synthetic-bearer', token_type: 'Bearer', patient: 'different-patient'});
    await expectAsync(result).toBeRejectedWithError(/expected synthetic patient/);
    http.expectNone(req => req.url.startsWith(metadata.url));
  });

  it('cancels an in-flight token exchange', async () => {
    const controller = new AbortController();
    const {result, auth} = await begin(controller.signal);
    callback({state: auth.searchParams.get('state'), code: 'synthetic-code'});
    await new Promise(resolve => setTimeout(resolve, 0));
    const request = http.expectOne(metadata.token_endpoint);
    controller.abort();
    await expectAsync(result).toBeRejectedWithError(/cancelled/);
    expect(request.cancelled).toBeTrue();
    expect(closed).toBeTrue();
  });
});
