import { ComponentFixture, TestBed } from '@angular/core/testing';

import { PatientProfileComponent } from './patient-profile.component';
import {FastenApiService} from '../../services/fasten-api.service';
import {of} from 'rxjs';
import {PipesModule} from '../../pipes/pipes.module';
import { ReportHeaderComponent } from 'src/app/components/report-header/report-header.component';
import { RouterTestingModule } from '@angular/router/testing';
import {NgbPopoverModule} from '@ng-bootstrap/ng-bootstrap';
import {FhirCardModule} from '../../components/fhir-card/fhir-card.module';
import {wrapFhirModel} from '../../../lib/models/factory';

describe('PatientProfileComponent', () => {
  let component: PatientProfileComponent;
  let fixture: ComponentFixture<PatientProfileComponent>;
  let mockedFastenApiService

  beforeEach(async () => {
    mockedFastenApiService = jasmine.createSpyObj('FastenApiService', ['getResources', 'getSummary'])
    await TestBed.configureTestingModule({
      declarations: [ PatientProfileComponent, ReportHeaderComponent ],
      imports: [PipesModule, RouterTestingModule, NgbPopoverModule, FhirCardModule],
      providers: [{
        provide: FastenApiService,
        useValue: mockedFastenApiService
      }]
    })
    .compileComponents();
    mockedFastenApiService.getResources.and.returnValue(of([]));
    mockedFastenApiService.getSummary.and.returnValue(of({sources: []}));
    fixture = TestBed.createComponent(PatientProfileComponent);
    component = fixture.componentInstance;
  });

  it('loads empty resource lists without passing invalid records to the FHIR factory', () => {
    fixture.detectChanges();
    expect(component.loading['page']).toBeFalse();
    expect(component.immunizations).toEqual([]);
    expect(component.allergyIntolerances).toEqual([]);
    expect(mockedFastenApiService.getResources).toHaveBeenCalledWith('Patient');
    expect(mockedFastenApiService.getResources).toHaveBeenCalledWith('Immunization');
    expect(mockedFastenApiService.getResources).toHaveBeenCalledWith('AllergyIntolerance');
  });

  it('renders synthetic patient, immunization, and allergy records', () => {
    const patient = wrapFhirModel({
      resourceType: 'Patient', id: 'synthetic-profile',
      name: [{family: 'Synthetic', given: ['Example']}]
    });
    const immunization = wrapFhirModel({
      resourceType: 'Immunization', id: 'synthetic-immunization', status: 'completed',
      vaccineCode: {text: 'Synthetic vaccine'},
      patient: {reference: 'Patient/synthetic-profile'}, occurrenceDateTime: '2026-01-01'
    });
    const allergy = wrapFhirModel({
      resourceType: 'AllergyIntolerance', id: 'synthetic-allergy',
      clinicalStatus: {coding: [{system: 'http://terminology.hl7.org/CodeSystem/allergyintolerance-clinical', code: 'active'}]},
      code: {text: 'Synthetic allergy'}, patient: {reference: 'Patient/synthetic-profile'}
    });
    mockedFastenApiService.getResources.withArgs('Patient').and.returnValue(of([patient]));
    mockedFastenApiService.getResources.withArgs('Immunization').and.returnValue(of([immunization]));
    mockedFastenApiService.getResources.withArgs('AllergyIntolerance').and.returnValue(of([allergy]));

    fixture.detectChanges();

    const view = fixture.nativeElement as HTMLElement;
    expect(view.querySelector('h1')?.textContent).toContain('Synthetic, Example');
    expect(view.textContent).toContain('Synthetic vaccine');
    expect(view.textContent).toContain('Synthetic allergy');
    expect(component.loading['page']).toBeFalse();
  });
});
