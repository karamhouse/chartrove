import { TestBed } from '@angular/core/testing';

import { FastenApiService } from './fasten-api.service';
import {HttpClientTestingModule, HttpTestingController} from '@angular/common/http/testing';
import {HTTP_CLIENT_TOKEN} from '../dependency-injection';
import {HttpClient} from '@angular/common/http';
import {DashboardWidgetQuery} from '../models/widget/dashboard-widget-query';

describe('FastenApiService', () => {
  let service: FastenApiService;

  beforeEach(() => {
    TestBed.configureTestingModule({
      imports: [HttpClientTestingModule],
      providers: [
        {
          provide: HTTP_CLIENT_TOKEN,
          useClass: HttpClient,
        },
      ]
    });
    service = TestBed.inject(FastenApiService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });

  it('imports a Patient through the local bundle API without provider credentials', async () => {
    const http = TestBed.inject(HttpTestingController);
    const result = service.importSandboxPatient({resourceType: 'Patient', id: 'synthetic-patient'}).toPromise();
    http.expectOne(req => req.url.endsWith('/secure/source')).flush({success: true, data: []});
    const request = http.expectOne(req => req.url.endsWith('/secure/source/manual'));
    const file = request.request.body.get('file') as File;
    expect(JSON.parse(await file.text())).toEqual({resourceType: 'Bundle', type: 'collection', entry: [{resource: {resourceType: 'Patient', id: 'synthetic-patient'}}]});
    request.flush({success: true, source: {id: 'local-source'}, data: {totalResources: 1}});
    expect((await result).source.id).toBe('local-source');
    http.verify();
  });

  it('reuses an existing synthetic import instead of creating duplicate local records', async () => {
    const http = TestBed.inject(HttpTestingController);
    const result = service.importSandboxPatient({resourceType: 'Patient', id: 'synthetic-patient'}).toPromise();
    http.expectOne(req => req.url.endsWith('/secure/source')).flush({success: true, data: [{id: 'already-local', patient: 'synthetic-patient', platform_type: 'manual'}]});
    expect(await result).toEqual({source: jasmine.objectContaining({id: 'already-local'}), alreadyImported: true});
    http.expectNone(req => req.url.endsWith('/secure/source/manual'));
    http.verify();
  });

});
