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
    const decoded = await new Response(request.request.body, {headers: {'Content-Type': request.request.headers.get('Content-Type')}}).formData();
    const file = decoded.get('file') as File;
    expect(JSON.parse(await file.text())).toEqual({resourceType: 'Bundle', type: 'collection', entry: [{resource: {resourceType: 'Patient', id: 'synthetic-patient'}}]});
    request.flush({success: true, source: {id: 'local-source'}, data: {totalResources: 1}});
    expect((await result).source.id).toBe('local-source');
    http.verify();
  });

  it('sends a serialized multipart Patient upload that the native transport can carry', async () => {
    const http = TestBed.inject(HttpTestingController);
    const patient = {resourceType: 'Patient' as const, id: 'synthetic-patient', name: [{family: 'López', given: ['Camila']}], text: {status: 'generated' as const, div: '<div>Line one\nLine two</div>'}};
    const result = service.importSandboxPatient(patient).toPromise();
    http.expectOne(req => req.url.endsWith('/secure/source')).flush({success: true, data: []});
    const request = http.expectOne(req => req.url.endsWith('/secure/source/manual'));
    const contentType = request.request.headers.get('Content-Type');
    expect(typeof request.request.body).toBe('string');
    expect(contentType).toMatch(/^multipart\/form-data; boundary=.+/);
    if (typeof request.request.body === 'string' && contentType) {
      const decoded = await new Response(request.request.body, {headers: {'Content-Type': contentType}}).formData();
      const file = decoded.get('file') as File;
      expect(file.name).toBe('epic-sandbox.json');
      expect(file.type).toBe('application/fhir+json');
      expect(JSON.parse(await file.text())).toEqual({resourceType: 'Bundle', type: 'collection', entry: [{resource: patient}]});
      const fields: string[] = [];
      decoded.forEach((_value, key) => fields.push(key));
      expect(fields).toEqual(['file']);
    }
    request.flush({success: true, source: {id: 'local-source'}});
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

  it('recognizes the pinned importer legacy patient key for the Epic sandbox patient', async () => {
    const http = TestBed.inject(HttpTestingController);
    const result = service.importSandboxPatient({resourceType: 'Patient', id: 'erXuFYUfucBZaryVksYEcMg3'}).toPromise();
    http.expectOne(req => req.url.endsWith('/secure/source')).flush({success: true, data: [
      {id: 'already-local', patient: 'rXuFYUfucBZaryVksYEcMg3', platform_type: 'manual'}
    ]});
    http.expectNone(req => req.url.endsWith('/secure/source/manual'));
    expect(await result).toEqual({source: jasmine.objectContaining({id: 'already-local'}), alreadyImported: true});
    http.verify();
  });

  it('does not reuse a direct provider snapshot as a legacy sandbox source', async () => {
    const http = TestBed.inject(HttpTestingController);
    const result = service.importSandboxPatient({resourceType: 'Patient', id: 'synthetic-patient'}).toPromise();
    http.expectOne(req => req.url.endsWith('/secure/source')).flush({success: true, data: [
      {id: 'direct-source', patient: 'synthetic-patient', platform_type: 'manual', endpoint_id: 'provider-scoped-id'}
    ]});
    http.expectOne(req => req.url.endsWith('/secure/source/manual')).flush({success: true, source: {id: 'sandbox-source'}});
    expect((await result).source.id).toBe('sandbox-source');
    http.verify();
  });

  it('sends a direct Patient snapshot with provider provenance and reads backend deduplication', async () => {
    const http = TestBed.inject(HttpTestingController);
    const patient = {resourceType: 'Patient' as const, id: 'patient-one'};
    const origin = {fhir_base_url: 'https://example.invalid/fhir', display: 'Example hospital'};
    const result = service.importSmartPatient(patient, origin).toPromise();
    http.expectNone(req => req.url.endsWith('/secure/source'));
    const request = http.expectOne(req => req.url.endsWith('/secure/source/manual'));
    const form = await new Response(request.request.body, {headers: {'Content-Type': request.request.headers.get('Content-Type')}}).formData();
    expect(JSON.parse(String(form.get('smart_origin')))).toEqual(origin);
    expect(JSON.parse(await (form.get('file') as File).text())).toEqual({resourceType: 'Bundle', type: 'collection', entry: [{resource: patient}]});
    request.flush({success: true, already_imported: true, source: {id: 'existing-direct-source'}});
    expect(await result).toEqual({source: jasmine.objectContaining({id: 'existing-direct-source'}), alreadyImported: true});
    http.verify();
  });

});
