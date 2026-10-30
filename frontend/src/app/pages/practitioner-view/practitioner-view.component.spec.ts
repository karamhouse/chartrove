import { ComponentFixture, TestBed } from '@angular/core/testing';
import { PractitionerViewComponent } from './practitioner-view.component';
import { ActivatedRoute } from '@angular/router';
import { of } from 'rxjs';
import { HTTP_CLIENT_TOKEN } from 'src/app/dependency-injection';
import { HttpClient, provideHttpClient, withInterceptorsFromDi, withXhr } from '@angular/common/http';
import {HttpTestingController, provideHttpClientTesting} from '@angular/common/http/testing';

describe('PractitionerViewComponent', () => {
  let component: PractitionerViewComponent;
  let fixture: ComponentFixture<PractitionerViewComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
    imports: [PractitionerViewComponent],
    providers: [
        {
            provide: ActivatedRoute,
            useValue: {
                snapshot: {
                    paramMap: { get: (key: string) => 'test-practitioner-id' },
                },
                params: of({ id: 'test-practitioner-id' }),
            },
        },
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
    fixture = TestBed.createComponent(PractitionerViewComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  afterEach(() => TestBed.inject(HttpTestingController).verify());

  it('reports a missing practitioner after a successful empty response', () => {
    const request = TestBed.inject(HttpTestingController).expectOne(req =>
      req.urlWithParams.endsWith('/api/secure/resource/fhir?sourceResourceType=Practitioner'));
    expect(request.request.method).toBe('GET');
    request.flush({success: true, data: []});
    fixture.detectChanges();
    expect(component.isLoading).toBeFalse();
    expect(component.loadError).toBe('Practitioner not found');
    expect(fixture.nativeElement.textContent).toContain('Practitioner not found');
  });
});
