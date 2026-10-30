import { Component, ChangeDetectionStrategy } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter, Router } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { of } from 'rxjs';
import { AuthService } from '../services/auth.service';
import { FastenApiService } from '../services/fasten-api.service';
import { EncryptionStatusGuard } from './encryption-status.guard';
import { IsAdminAuthGuard } from './is-admin-auth-guard';
import { IsAuthenticatedAuthGuard } from './is-authenticated-auth-guard';

@Component({ standalone: true, changeDetection: ChangeDetectionStrategy.Eager,
 template: '<p>Protected synthetic record</p>' })
class ProtectedPage {
  static activations = 0;

  constructor() {
    ProtectedPage.activations++;
  }
}

@Component({ standalone: true, changeDetection: ChangeDetectionStrategy.Eager,
 template: '<p>Public destination</p>' })
class PublicPage {}

describe('Route guard integration', () => {
  let auth: jasmine.SpyObj<AuthService>;
  let api: jasmine.SpyObj<FastenApiService>;
  let harness: RouterTestingHarness;

  beforeEach(async () => {
    ProtectedPage.activations = 0;
    auth = jasmine.createSpyObj<AuthService>('AuthService', ['IsAuthenticated', 'IsAdmin']);
    auth.IsAuthenticated.and.returnValue(Promise.resolve(true));
    auth.IsAdmin.and.returnValue(false);
    api = jasmine.createSpyObj<FastenApiService>('FastenApiService', ['getHealth']);
    api.getHealth.and.returnValue(of({ standby_mode: false, first_run_wizard: false }));

    TestBed.configureTestingModule({
      providers: [
        { provide: AuthService, useValue: auth },
        { provide: FastenApiService, useValue: api },
        IsAuthenticatedAuthGuard,
        provideRouter([
          { path: 'records', component: ProtectedPage, canActivate: [IsAuthenticatedAuthGuard] },
          { path: 'admin', component: ProtectedPage, canActivate: [IsAdminAuthGuard] },
          { path: 'ready', component: ProtectedPage, canActivate: [EncryptionStatusGuard] },
          { path: 'auth/signin', component: PublicPage },
          { path: 'dashboard', component: PublicPage },
          { path: 'encryption-key/wizard', component: PublicPage },
          { path: 'encryption-key/wizard-restore', component: PublicPage },
          { path: 'auth/signup/wizard', component: PublicPage },
        ]),
      ],
    });
    harness = await RouterTestingHarness.create();
  });

  async function expectRedirect(from: string, to: string) {
    await harness.navigateByUrl(from, PublicPage);
    expect(TestBed.inject(Router).url).toBe(to);
    expect(ProtectedPage.activations).toBe(0);
    expect(harness.routeNativeElement.textContent).toContain('Public destination');
  }

  it('redirects signed-out users before a protected record component is created', async () => {
    auth.IsAuthenticated.and.returnValue(Promise.resolve(false));
    await expectRedirect('/records', '/auth/signin');
  });

  it('renders a protected route for an authenticated user', async () => {
    await harness.navigateByUrl('/records', ProtectedPage);
    expect(TestBed.inject(Router).url).toBe('/records');
    expect(ProtectedPage.activations).toBe(1);
  });

  it('rejects an unauthenticated user even if an admin flag is present', async () => {
    auth.IsAuthenticated.and.returnValue(Promise.resolve(false));
    auth.IsAdmin.and.returnValue(true);
    await expectRedirect('/admin', '/dashboard');
  });

  it('rejects an authenticated non-admin before the admin component is created', async () => {
    await expectRedirect('/admin', '/dashboard');
  });

  it('allows an authenticated administrator into the admin route', async () => {
    auth.IsAdmin.and.returnValue(true);
    await harness.navigateByUrl('/admin', ProtectedPage);
    expect(TestBed.inject(Router).url).toBe('/admin');
    expect(ProtectedPage.activations).toBe(1);
  });

  for (const scenario of [
    { standby: true, firstRun: true, destination: '/encryption-key/wizard' },
    { standby: true, firstRun: false, destination: '/encryption-key/wizard-restore' },
    { standby: false, firstRun: true, destination: '/auth/signup/wizard' },
  ]) {
    it(`redirects to ${scenario.destination} before rendering records`, async () => {
      api.getHealth.and.returnValue(of({
        standby_mode: scenario.standby,
        first_run_wizard: scenario.firstRun,
      }));
      await expectRedirect('/ready', scenario.destination);
    });
  }

  it('renders records when database setup is complete and it is unlocked', async () => {
    await harness.navigateByUrl('/ready', ProtectedPage);
    expect(TestBed.inject(Router).url).toBe('/ready');
    expect(ProtectedPage.activations).toBe(1);
  });
});
