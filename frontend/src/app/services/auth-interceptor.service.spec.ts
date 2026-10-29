import { HttpHandler, HttpRequest, HttpResponse } from '@angular/common/http';
import {Router} from '@angular/router';
import {of} from 'rxjs';
import {AuthInterceptorService} from './auth-interceptor.service';
import {AuthService} from './auth.service';

describe('AuthInterceptorService Patient privacy', () => {
  it('does not log a serialized Patient import or its authorization header', () => {
    const log = spyOn(console, 'log');
    const auth = jasmine.createSpyObj<AuthService>('AuthService', ['GetAuthToken']);
    auth.GetAuthToken.and.returnValue('synthetic-test-token');
    const handler = jasmine.createSpyObj<HttpHandler>('HttpHandler', ['handle']);
    handler.handle.and.returnValue(of(new HttpResponse({status: 200})));
    const interceptor = new AuthInterceptorService(auth, jasmine.createSpyObj<Router>('Router', ['navigateByUrl']));
    const body = '--synthetic-boundary\r\n{"resourceType":"Patient","id":"synthetic-only","name":[{"text":"Synthetic Patient"}]}';
    interceptor.intercept(new HttpRequest('POST', '/api/secure/source/manual', body), handler).subscribe();
    expect(handler.handle).toHaveBeenCalledTimes(1);
    expect(log).not.toHaveBeenCalled();
  });
});
