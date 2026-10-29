import { ComponentFixture, TestBed } from '@angular/core/testing';
import { PractitionerCreateComponent } from './practitioner-create.component';
import { HTTP_CLIENT_TOKEN } from 'src/app/dependency-injection';
import { HttpClient, provideHttpClient, withInterceptorsFromDi, withXhr } from '@angular/common/http';
import {HttpTestingController, provideHttpClientTesting} from '@angular/common/http/testing';

describe('PractitionerCreateComponent', () => {
  let component: PractitionerCreateComponent;
  let fixture: ComponentFixture<PractitionerCreateComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
    imports: [PractitionerCreateComponent],
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
    fixture = TestBed.createComponent(PractitionerCreateComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  afterEach(() => TestBed.inject(HttpTestingController).verify());

  it('initializes a new practitioner form with an empty count response', () => {
    const request = TestBed.inject(HttpTestingController).expectOne(req => req.url.endsWith('/api/secure/query'));
    expect(request.request.method).toBe('POST');
    expect(request.request.body.from).toBe('Practitioner');
    request.flush({success: true, data: []});
    fixture.detectChanges();
    expect(component.totalPractitioners).toBe(0);
  });
});
