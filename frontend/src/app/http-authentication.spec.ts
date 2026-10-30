import {HttpClient, HttpErrorResponse} from '@angular/common/http';
import {HttpTestingController, provideHttpClientTesting} from '@angular/common/http/testing';
import {TestBed} from '@angular/core/testing';
import {Router} from '@angular/router';
import {AppModule} from './app.module';
import {HTTP_CLIENT_TOKEN} from './dependency-injection';
import {AuthService} from './services/auth.service';

describe('Application HTTP authentication providers', () => {
  let http: HttpTestingController;
  let auth: jasmine.SpyObj<AuthService>;
  let router: jasmine.SpyObj<Router>;

  beforeEach(() => {
    auth = jasmine.createSpyObj<AuthService>('AuthService', ['GetAuthToken', 'Logout']);
    auth.GetAuthToken.and.returnValue('synthetic-provider-test-token');
    router = jasmine.createSpyObj<Router>('Router', ['navigateByUrl']);
    router.navigateByUrl.and.returnValue(Promise.resolve(true));
    TestBed.configureTestingModule({
      imports: [AppModule],
      providers: [
        provideHttpClientTesting(),
        {provide: AuthService, useValue: auth},
        {provide: Router, useValue: router},
      ],
    });
    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => http.verify());

  for (const {name, getClient} of [
    {name: 'HttpClient', getClient: () => TestBed.inject(HttpClient)},
    {name: 'HTTP_CLIENT_TOKEN', getClient: () => TestBed.inject(HTTP_CLIENT_TOKEN) as unknown as HttpClient},
  ]) {
    it(`authenticates local API requests through ${name}`, () => {
      const client = getClient();
      const body = {resourceType: 'Patient', id: 'synthetic-only'};
      client.post('/api/secure/source/manual', body).subscribe();
      const request = http.expectOne('/api/secure/source/manual');
      expect(request.request.headers.get('Authorization')).toBe('Bearer synthetic-provider-test-token');
      expect(request.request.body).toEqual(body);
      request.flush({success: true});
    });

    it(`keeps the session token off unrelated origins through ${name}`, () => {
      const client = getClient();
      const url = 'https://synthetic.example.invalid/api/secure/source/manual';
      client.get(url).subscribe();
      const request = http.expectOne(url);
      expect(request.request.headers.has('Authorization')).toBeFalse();
      request.flush({});
      expect(auth.GetAuthToken).not.toHaveBeenCalled();
    });
  }

  for (const status of [401, 403]) {
    it(`ends the session and redirects after an API ${status}`, () => {
      TestBed.inject(HttpClient).get('/api/secure/summary').subscribe();
      http.expectOne('/api/secure/summary').flush({}, {status, statusText: 'Denied'});
      expect(auth.Logout).toHaveBeenCalledTimes(1);
      expect(router.navigateByUrl).toHaveBeenCalledTimes(1);
      expect(router.navigateByUrl).toHaveBeenCalledWith('/auth/signin');
    });
  }

  it('propagates server errors without discarding the session', () => {
    let error: HttpErrorResponse | undefined;
    TestBed.inject(HttpClient).get('/api/secure/summary').subscribe({
      next: () => fail('A server error must not become a successful response'),
      error: value => error = value,
    });
    http.expectOne('/api/secure/summary').flush({}, {status: 500, statusText: 'Synthetic failure'});
    expect(error?.status).toBe(500);
    expect(auth.Logout).not.toHaveBeenCalled();
    expect(router.navigateByUrl).not.toHaveBeenCalled();
  });
});
