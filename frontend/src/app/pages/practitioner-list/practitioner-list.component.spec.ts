import { ComponentFixture, TestBed } from '@angular/core/testing';

import { PractitionerListComponent } from './practitioner-list.component';
import { HTTP_CLIENT_TOKEN } from 'src/app/dependency-injection';
import { HttpClient, provideHttpClient, withInterceptorsFromDi, withXhr } from '@angular/common/http';
import {HttpTestingController, provideHttpClientTesting} from '@angular/common/http/testing';

describe('PractitionerListComponent', () => {
  let component: PractitionerListComponent;
  let fixture: ComponentFixture<PractitionerListComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
    imports: [PractitionerListComponent],
    providers: [
        {
            provide: HTTP_CLIENT_TOKEN,
            useClass: HttpClient,
        },
        provideHttpClient(withXhr(), withInterceptorsFromDi()),
        provideHttpClientTesting(),
    ]
}).compileComponents();
  });

  beforeEach(() => {
    fixture = TestBed.createComponent(PractitionerListComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  afterEach(() => TestBed.inject(HttpTestingController).verify());

  it('finishes loading an empty practitioner list', () => {
    const request = TestBed.inject(HttpTestingController).expectOne(req =>
      req.urlWithParams.endsWith('/api/secure/resource/fhir?sourceResourceType=Practitioner'));
    expect(request.request.method).toBe('GET');
    request.flush({success: true, data: []});
    const favorites = TestBed.inject(HttpTestingController).expectOne(req =>
      req.urlWithParams.endsWith('/api/secure/user/favorites?resource_type=Practitioner'));
    expect(favorites.request.method).toBe('GET');
    favorites.flush({success: true, data: []});
    fixture.detectChanges();
    expect(component.loading).toBeFalse();
    expect(component.practitioners).toEqual([]);
  });
});
