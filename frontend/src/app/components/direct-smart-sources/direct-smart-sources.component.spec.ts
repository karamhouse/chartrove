import {ComponentFixture, TestBed} from '@angular/core/testing';
import {CommonModule} from '@angular/common';
import {RouterTestingModule} from '@angular/router/testing';
import {of, Subject, throwError} from 'rxjs';
import {DirectSmartSourcesComponent} from './direct-smart-sources.component';
import {DirectSmartService} from '../../services/direct-smart.service';
import {FastenApiService} from '../../services/fasten-api.service';
import {Source} from '../../models/fasten/source';

describe('DirectSmartSourcesComponent', () => {
  let fixture: ComponentFixture<DirectSmartSourcesComponent>;
  let component: DirectSmartSourcesComponent;
  let direct: jasmine.SpyObj<DirectSmartService>;
  let api: jasmine.SpyObj<FastenApiService>;
  const provider = {id: 'epic-one', display: 'Example hospital', fhir_base_url: 'https://example.invalid/fhir'};
  const connection = {id: 'handle-one', provider_id: 'epic-one', patient_id: 'patient-one', expires_at: 9999999999};
  const patient = {resourceType: 'Patient' as const, id: 'patient-one', name: [{text: '<img src=x onerror=alert(1)>'}], birthDate: '2000-01-02'};

  beforeEach(async () => {
    direct = jasmine.createSpyObj<DirectSmartService>('DirectSmartService', ['providers', 'connect', 'disconnect']);
    direct.providers.and.resolveTo([provider]);
    direct.connect.and.resolveTo({connection, patient});
    direct.disconnect.and.resolveTo();
    api = jasmine.createSpyObj<FastenApiService>('FastenApiService', ['importSmartPatient']);
    api.importSmartPatient.and.returnValue(of({source: new Source({id: 'source-one'}), alreadyImported: false}));
    await TestBed.configureTestingModule({declarations: [DirectSmartSourcesComponent], imports: [CommonModule, RouterTestingModule],
      providers: [{provide: DirectSmartService, useValue: direct}, {provide: FastenApiService, useValue: api}]}).compileComponents();
    fixture = TestBed.createComponent(DirectSmartSourcesComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  });

  it('previews escaped Patient data and writes nothing before explicit import', async () => {
    fixture.nativeElement.querySelector('[data-testid="direct-smart-connect"]').click();
    await fixture.whenStable(); fixture.detectChanges();
    const preview = fixture.nativeElement.querySelector('[data-testid="direct-patient-preview"]');
    expect(preview.textContent).toContain('patient-one');
    expect(preview.textContent).toContain('<img src=x onerror=alert(1)>');
    expect(preview.querySelector('img')).toBeNull();
    expect(api.importSmartPatient).not.toHaveBeenCalled();
    fixture.nativeElement.querySelector('[data-testid="direct-smart-import"]').click();
    await fixture.whenStable(); fixture.detectChanges();
    expect(api.importSmartPatient).toHaveBeenCalledWith(patient, {fhir_base_url: provider.fhir_base_url, display: provider.display});
    expect(direct.disconnect).toHaveBeenCalledWith('handle-one');
    expect(fixture.nativeElement.textContent).toContain('View imported Patient');
    expect(fixture.nativeElement.querySelector('[data-testid="direct-patient-preview"]')).toBeNull();
  });

  it('keeps a preview after import failure and permits explicit retry without another connection', async () => {
    await component.connect();
    api.importSmartPatient.and.returnValues(throwError(new Error('private body')), of({source: new Source({id: 'source-one'}), alreadyImported: true}));
    await component.importPatient(); fixture.detectChanges();
    expect(fixture.nativeElement.textContent).toContain('retry without signing in again');
    expect(fixture.nativeElement.textContent).not.toContain('private body');
    expect(component.patient).toBeDefined();
    await component.importPatient(); fixture.detectChanges();
    expect(direct.connect).toHaveBeenCalledTimes(1);
    expect(fixture.nativeElement.textContent).toContain('already imported');
  });

  it('cancels and releases a late result when the view is destroyed', async () => {
    let finish: (result: {connection: typeof connection, patient: typeof patient}) => void;
    direct.connect.and.callFake(() => new Promise(resolve => finish = resolve));
    const pending = component.connect();
    const signal = direct.connect.calls.mostRecent().args[1];
    fixture.destroy();
    expect(signal.aborted).toBeTrue();
    finish({connection, patient});
    await pending;
    expect(direct.disconnect).toHaveBeenCalledWith('handle-one');
    expect(api.importSmartPatient).not.toHaveBeenCalled();
    expect(component.patient).toBeUndefined();
  });

  it('shows an actionable empty state without opening authorization', async () => {
    direct.providers.and.resolveTo([]);
    await component.loadProviders(); fixture.detectChanges();
    expect(fixture.nativeElement.textContent).toContain('No direct providers are configured');
    expect(fixture.nativeElement.querySelector('[data-testid="direct-smart-connect"]')).toBeNull();
    expect(direct.connect).not.toHaveBeenCalled();
  });

  it('discards a preview without importing it', async () => {
    await component.connect();
    await component.disconnect(); fixture.detectChanges();
    expect(component.patient).toBeUndefined();
    expect(direct.disconnect).toHaveBeenCalledWith('handle-one');
    expect(api.importSmartPatient).not.toHaveBeenCalled();
  });

  it('does not send a second import while confirmation is pending', async () => {
    await component.connect();
    const result = new Subject<{source: Source, alreadyImported: boolean}>();
    api.importSmartPatient.and.returnValue(result);
    const first = component.importPatient();
    const second = component.importPatient();
    expect(api.importSmartPatient).toHaveBeenCalledTimes(1);
    result.next({source: new Source({id: 'source-one'}), alreadyImported: false}); result.complete();
    await Promise.all([first, second]);
    expect(component.importing).toBeFalse();
  });
});
