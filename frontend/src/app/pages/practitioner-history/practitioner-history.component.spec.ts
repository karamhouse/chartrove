import { ComponentFixture, TestBed } from '@angular/core/testing';

import { PractitionerHistoryComponent } from './practitioner-history.component';
import { RouterTestingModule } from '@angular/router/testing';
import { of } from 'rxjs';
import { ReportHeaderComponent } from 'src/app/components/report-header/report-header.component';
import { ActivatedRoute } from '@angular/router';
import { HTTP_CLIENT_TOKEN } from 'src/app/dependency-injection';
import { HttpClient, provideHttpClient, withInterceptorsFromDi } from '@angular/common/http';
import { MedicalHistoryComponent } from '../medical-history/medical-history.component';
import {FastenApiService} from '../../services/fasten-api.service';

describe('PractitionerHistoryComponent', () => {
  let mockedFastenApiService;
  let component: PractitionerHistoryComponent;
  let fixture: ComponentFixture<PractitionerHistoryComponent>;

  beforeEach(async () => {
    mockedFastenApiService = jasmine.createSpyObj('FastenApiService', [
      'getResources',
      'getPractitionerHistory',
      'getResourceGraph',
      'getSummary',
    ]);
    await TestBed.configureTestingModule({
    declarations: [PractitionerHistoryComponent, ReportHeaderComponent, MedicalHistoryComponent],
    imports: [RouterTestingModule],
    providers: [
        {provide: FastenApiService, useValue: mockedFastenApiService},
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
        provideHttpClient(withInterceptorsFromDi()),
    ]
}).compileComponents();
    mockedFastenApiService.getResourceGraph.and.returnValue(
      of({ Condition: [], Encounter: [] })
    );
    mockedFastenApiService.getResources.and.returnValue(of([]));
    mockedFastenApiService.getPractitionerHistory.and.returnValue(of([]));
    mockedFastenApiService.getSummary.and.returnValue(of({ sources: [] }));

    fixture = TestBed.createComponent(PractitionerHistoryComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('finishes loading when the practitioner has no encounters', () => {
    expect(component.loading).toBeFalse();
    expect(mockedFastenApiService.getPractitionerHistory).toHaveBeenCalledWith('test-practitioner-id');
  });
});
