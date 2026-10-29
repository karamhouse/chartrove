import { ComponentFixture, TestBed } from '@angular/core/testing';

import { DesktopCallbackComponent, readDesktopAuthCallback } from './desktop-callback.component';
import { provideHttpClientTesting } from '@angular/common/http/testing';
import {RouterTestingModule} from '@angular/router/testing';
import { LoadingSpinnerComponent } from 'src/app/components/loading-spinner/loading-spinner.component';
import { provideHttpClient, withInterceptorsFromDi, withXhr } from '@angular/common/http';

describe('DesktopCallbackComponent', () => {
  let component: DesktopCallbackComponent;
  let fixture: ComponentFixture<DesktopCallbackComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
    declarations: [DesktopCallbackComponent],
    imports: [RouterTestingModule, LoadingSpinnerComponent],
    providers: [provideHttpClient(withXhr(), withInterceptorsFromDi()), provideHttpClientTesting()]
})
    .compileComponents();

    fixture = TestBed.createComponent(DesktopCallbackComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });

  it('reads OAuth query parameters before the desktop hash route', () => {
    expect(readDesktopAuthCallback('test-state', 'wails://localhost/?code=synthetic-code&state=test-state#desktop/callback/test-state'))
      .toEqual({state: 'test-state', code: 'synthetic-code'});
  });

  it('reads OAuth query parameters inside the desktop hash route', () => {
    expect(readDesktopAuthCallback('test-state', 'wails://localhost/#/desktop/callback/test-state?code=synthetic-code&state=test-state'))
      .toEqual({state: 'test-state', code: 'synthetic-code'});
  });

  it('rejects conflicting states and drops unrelated credential parameters', () => {
    expect(() => readDesktopAuthCallback('test-state', 'wails://localhost/?code=synthetic-code&state=other#desktop/callback/test-state')).toThrow();
    expect(readDesktopAuthCallback('test-state', 'wails://localhost/?code=synthetic-code&state=test-state&access_token=discarded#desktop/callback/test-state'))
      .toEqual({state: 'test-state', code: 'synthetic-code'});
  });
});
