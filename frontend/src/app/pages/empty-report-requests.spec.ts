import { HttpClient } from '@angular/common/http';
import { HttpClientTestingModule, HttpTestingController } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { ActivatedRoute } from '@angular/router';
import { RouterTestingModule } from '@angular/router/testing';
import { NgbModal } from '@ng-bootstrap/ng-bootstrap';
import { of } from 'rxjs';
import { HTTP_CLIENT_TOKEN } from '../dependency-injection';
import { ResourceFhir } from '../models/fasten/resource_fhir';
import { FastenApiService } from '../services/fasten-api.service';
import { MedicalHistoryComponent } from './medical-history/medical-history.component';
import { ReportLabsComponent } from './report-labs/report-labs.component';

describe('Report requests for empty pages', () => {
  let http: HttpTestingController;
  let api: FastenApiService;
  const encounter = new ResourceFhir({
    source_id: 'synthetic-source', source_resource_type: 'Encounter',
    source_resource_id: 'synthetic-encounter',
    resource_raw: { resourceType: 'Encounter', id: 'synthetic-encounter' },
  });

  beforeEach(() => {
    TestBed.configureTestingModule({
      imports: [HttpClientTestingModule, RouterTestingModule],
      providers: [{ provide: HTTP_CLIENT_TOKEN, useExisting: HttpClient }],
    });
    http = TestBed.inject(HttpTestingController);
    api = TestBed.inject(FastenApiService);
  });

  afterEach(() => http.verify());

  it('finishes loading a new account history without requesting an empty graph', () => {
    const component = new MedicalHistoryComponent(api, TestBed.inject(NgbModal));
    component.ngOnInit();
    http.expectOne(req => req.params.get('sourceResourceType') === 'Encounter')
      .flush({ success: true, data: [] });

    http.expectNone(req => req.url.includes('/resource/graph/'));
    expect(component.loading).toBeFalse();
    expect(component.encounters).toEqual([]);
  });

  it('clears previous encounters when the selected history page is empty', () => {
    const component = new MedicalHistoryComponent(api, TestBed.inject(NgbModal));
    component.encounters = [encounter];
    component.allEncountersIds = [encounter];
    component.currentPage = 2;
    component.pageChange();

    http.expectNone(req => req.url.includes('/resource/graph/'));
    expect(component.loading).toBeFalse();
    expect(component.encounters).toEqual([]);
  });

  it('still fetches and displays the graph for a populated history page', () => {
    const component = new MedicalHistoryComponent(api, TestBed.inject(NgbModal));
    component.ngOnInit();
    http.expectOne(req => req.params.get('sourceResourceType') === 'Encounter')
      .flush({ success: true, data: [encounter] });
    const graph = http.expectOne(req => req.url.endsWith('/resource/graph/MedicalHistory'));
    expect(graph.request.body.resource_ids).toEqual([{
      source_id: encounter.source_id, source_resource_type: 'Encounter',
      source_resource_id: encounter.source_resource_id,
    }]);
    graph.flush({ success: true, data: { results: { Encounter: [encounter] } } });

    expect(component.loading).toBeFalse();
    expect(component.encounters).toEqual([encounter]);
  });

  function labs() {
    return new ReportLabsComponent(api, { params: of({}) } as ActivatedRoute);
  }

  it('finishes loading new account labs without submitting an empty code filter', () => {
    const component = labs();
    component.ngOnInit();
    http.expectOne(req => req.body?.from === 'DiagnosticReport')
      .flush({ success: true, data: [] });
    http.expectOne(req => req.body?.from === 'Observation' && !!req.body.aggregations)
      .flush({ success: true, data: [] });

    http.expectNone(req => req.url.endsWith('/secure/query'));
    expect(component.loading).toBeFalse();
    expect(component.isEmptyReport).toBeTrue();
    expect(component.observationGroups).toEqual({});
  });

  it('clears previous lab groups and titles when the selected page is empty', () => {
    const component = labs();
    component.allObservationGroups = ['http://loinc.org|synthetic-code'];
    component.observationGroups = { 'synthetic-code': [] };
    component.observationGroupTitles = { 'synthetic-code': 'Synthetic lab' };
    component.currentPage = 2;
    component.populateObservationsForCurrentPage();

    http.expectNone(req => req.url.endsWith('/secure/query'));
    expect(component.loading).toBeFalse();
    expect(component.isEmptyReport).toBeTrue();
    expect(component.observationGroups).toEqual({});
    expect(component.observationGroupTitles).toEqual({});
  });

  it('still filters and groups lab observations on a populated page', () => {
    const component = labs();
    component.allObservationGroups = ['http://loinc.org|synthetic-code'];
    component.populateObservationsForCurrentPage();
    const request = http.expectOne(req => req.url.endsWith('/secure/query'));
    expect(request.request.body.where.code).toBe('http://loinc.org|synthetic-code');
    const observation = new ResourceFhir({
      source_id: 'synthetic-source', source_resource_type: 'Observation',
      source_resource_id: 'synthetic-observation',
      resource_raw: {
        resourceType: 'Observation',
        code: { coding: [{ system: 'http://loinc.org', code: 'synthetic-code', display: 'Synthetic lab' }] },
      },
    });
    request.flush({ success: true, data: [observation] });

    expect(component.loading).toBeFalse();
    expect(component.isEmptyReport).toBeFalse();
    expect(component.observationGroups['synthetic-code']).toEqual([observation]);
    expect(component.observationGroupTitles['synthetic-code']).toBe('Synthetic lab');
  });
});
