import { HttpClient, provideHttpClient, withInterceptorsFromDi, withXhr } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { fakeAsync, TestBed, tick } from '@angular/core/testing';
import { Router } from '@angular/router';
import { RouterTestingModule } from '@angular/router/testing';
import { NgbActiveModal } from '@ng-bootstrap/ng-bootstrap';
import { HTTP_CLIENT_TOKEN } from '../dependency-injection';
import { PractitionerCreateComponent } from '../pages/practitioner-create/practitioner-create.component';
import { FastenApiService } from '../services/fasten-api.service';
import { MedicalRecordWizardAddEncounterComponent } from './medical-record-wizard-add-encounter/medical-record-wizard-add-encounter.component';
import { MedicalRecordWizardAddOrganizationComponent } from './medical-record-wizard-add-organization/medical-record-wizard-add-organization.component';
import { MedicalRecordWizardAddPractitionerComponent } from './medical-record-wizard-add-practitioner/medical-record-wizard-add-practitioner.component';

describe('Record entry resource counts', () => {
  const screens = [
    {
      name: 'encounter dialog', resource: 'Encounter', count: 'totalEncounters',
      create: (api: FastenApiService) => new MedicalRecordWizardAddEncounterComponent(new NgbActiveModal(), api),
    },
    {
      name: 'organization dialog', resource: 'Organization', count: 'totalOrganizations',
      create: (api: FastenApiService) => new MedicalRecordWizardAddOrganizationComponent(new NgbActiveModal(), api),
    },
    {
      name: 'practitioner dialog', resource: 'Practitioner', count: 'totalPractitioners',
      create: (api: FastenApiService) => new MedicalRecordWizardAddPractitionerComponent(new NgbActiveModal(), api),
    },
    {
      name: 'practitioner page', resource: 'Practitioner', count: 'totalPractitioners',
      create: (api: FastenApiService) => new PractitionerCreateComponent(TestBed.inject(Router), api),
    },
  ];
  const responses = [
    { name: 'empty query result', body: { success: true, data: [] }, expected: 0 },
    { name: 'null query data', body: { success: true, data: null }, expected: 0 },
    { name: 'omitted query data', body: { success: true }, expected: 0 },
    { name: 'missing count value', body: { success: true, data: [{}] }, expected: 0 },
    { name: 'zero count', body: { success: true, data: [{ value: 0 }] }, expected: 0 },
    { name: 'existing records', body: { success: true, data: [{ value: 7 }] }, expected: 7 },
  ];

  beforeEach(() => {
    TestBed.configureTestingModule({
    imports: [RouterTestingModule],
    providers: [{ provide: HTTP_CLIENT_TOKEN, useExisting: HttpClient }, provideHttpClient(withXhr(), withInterceptorsFromDi()), provideHttpClientTesting()]
});
  });

  afterEach(() => TestBed.inject(HttpTestingController).verify());

  for (const screen of screens) {
    for (const response of responses) {
      it(`${screen.name} handles ${response.name}`, fakeAsync(() => {
        const component = screen.create(TestBed.inject(FastenApiService));
        component[screen.count] = 99;
        component.ngOnInit();

        const request = TestBed.inject(HttpTestingController).expectOne(req =>
          req.method === 'POST' && req.url.endsWith('/secure/query') && req.body.from === screen.resource);
        request.flush(response.body);
        tick();

        expect(component[screen.count]).toBe(response.expected);
      }));
    }
  }
});
