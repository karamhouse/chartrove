import { ComponentFixture, TestBed } from '@angular/core/testing';

import { SourceDetailComponent } from './source-detail.component';
import { provideHttpClientTesting } from '@angular/common/http/testing';
import {RouterTestingModule} from '@angular/router/testing';
import {ActivatedRoute, convertToParamMap, RouterModule} from '@angular/router';
import {HTTP_CLIENT_TOKEN} from '../../dependency-injection';
import { HttpClient, provideHttpClient, withInterceptorsFromDi } from '@angular/common/http';
import { FhirDatatableModule } from 'src/app/components/fhir-datatable/fhir-datatable.module';
import {ResourceFhir} from '../../models/fasten/resource_fhir';

describe('SourceDetailComponent', () => {
  let component: SourceDetailComponent;
  let fixture: ComponentFixture<SourceDetailComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
    declarations: [SourceDetailComponent],
    imports: [RouterTestingModule, RouterModule, FhirDatatableModule],
    providers: [
        {
            provide: ActivatedRoute,
            useValue: { snapshot: { paramMap: convertToParamMap({ 'source_id': 'b64.c291cmNlOmF0aGVuYTphLTgwMDAwLkUtMTQ1NDU' }) } }
        },
        {
            provide: HTTP_CLIENT_TOKEN,
            useClass: HttpClient,
        },
        provideHttpClient(withInterceptorsFromDi()),
        provideHttpClientTesting(),
    ]
})
    .compileComponents();

    fixture = TestBed.createComponent(SourceDetailComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });

  it('displays a text-only SMART Patient name and leaves missing addresses empty', () => {
    component.selectedPatient = new ResourceFhir({resource_raw: {resourceType: 'Patient', id: 'synthetic-only',
      name: [{text: 'Chartrove Synthetic Patient'}]}});
    expect(component.getPatientName()).toBe('Chartrove Synthetic Patient');
    expect(component.getPatientAddress()).toEqual([]);
  });

  it('keeps structured names and partial addresses readable', () => {
    component.selectedPatient = new ResourceFhir({resource_raw: {name: [{given: ['Synthetic'], family: 'Patient'}],
      address: [{line: ['Synthetic Street'], city: 'Example City'}]}});
    expect(component.getPatientName()).toBe('Patient, Synthetic');
    expect(component.getPatientAddress()).toEqual(['Synthetic Street', 'Example City']);
  });

  it('uses a clear fallback when a Patient has no name', () => {
    component.selectedPatient = new ResourceFhir({resource_raw: {resourceType: 'Patient', id: 'synthetic-only'}});
    expect(component.getPatientName()).toBe('Name not provided');
  });
});
