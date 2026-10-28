import { ComponentFixture, TestBed } from '@angular/core/testing';

import { MedicalSourcesComponent } from './medical-sources.component';
import {HttpClientTestingModule} from '@angular/common/http/testing';
import {RouterTestingModule} from '@angular/router/testing';
import {HTTP_CLIENT_TOKEN} from '../../dependency-injection';
import {HttpClient} from '@angular/common/http';
import {FormsModule, ReactiveFormsModule} from '@angular/forms';
import { LoadingSpinnerComponent } from 'src/app/components/loading-spinner/loading-spinner.component';
import { MedicalSourcesFilterComponent } from 'src/app/components/medical-sources-filter/medical-sources-filter.component';
import { MedicalSourcesConnectedComponent } from 'src/app/components/medical-sources-connected/medical-sources-connected.component';
import { NgxDropzoneModule } from 'ngx-dropzone';
import {EpicSandboxService} from '../../services/epic-sandbox.service';
import {FastenApiService} from '../../services/fasten-api.service';
import {of, throwError} from 'rxjs';
import {Source} from '../../models/fasten/source';
import {DirectSmartSourcesComponent} from '../../components/direct-smart-sources/direct-smart-sources.component';
import {DirectSmartService} from '../../services/direct-smart.service';

describe('MedicalSourcesComponent', () => {
  let component: MedicalSourcesComponent;
  let fixture: ComponentFixture<MedicalSourcesComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      declarations: [ MedicalSourcesComponent, MedicalSourcesFilterComponent, MedicalSourcesConnectedComponent, DirectSmartSourcesComponent ],
      imports: [HttpClientTestingModule, RouterTestingModule, FormsModule, ReactiveFormsModule, LoadingSpinnerComponent, NgxDropzoneModule],
      providers: [
        {provide: DirectSmartService, useValue: {providers: async () => []}},
        {
          provide: HTTP_CLIENT_TOKEN,
          useClass: HttpClient,
        },
      ]
    })
    .compileComponents();

    fixture = TestBed.createComponent(MedicalSourcesComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });

  it('offers the Epic sandbox connection separately from production providers', () => {
    component.environment_name = 'desktop_prod';
    fixture.detectChanges();
    const button = fixture.nativeElement.querySelector('[data-testid="connect-epic-sandbox"]');
    expect(button).not.toBeNull();
    expect(fixture.nativeElement.textContent).toContain('Synthetic test data only');
  });

  it('shows progress and enables import after retrieving a synthetic Patient', async () => {
    component.environment_name = 'desktop_prod';
    let complete: (patient: any) => void;
    spyOn(TestBed.inject(EpicSandboxService), 'connect').and.callFake((_signal, progress) => {
      progress('Waiting for sandbox sign-in');
      return new Promise(resolve => complete = resolve);
    });
    const connect = spyOn(component, 'connectEpicSandbox').and.callThrough();
    fixture.detectChanges();
    fixture.nativeElement.querySelector('[data-testid="connect-epic-sandbox"]').click();
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('[data-testid="connect-epic-sandbox"]').disabled).toBeTrue();
    expect(fixture.nativeElement.textContent).toContain('Waiting for sandbox sign-in');
    complete({resourceType: 'Patient', id: 'synthetic-patient'});
    await connect.calls.mostRecent().returnValue;
    await fixture.whenStable();
    fixture.detectChanges();
    expect(fixture.nativeElement.textContent).toContain('Import synthetic Patient');
  });

  it('allows retrying a failed local import without another provider sign-in', async () => {
    component.environment_name = 'desktop_prod';
    component.sandboxPatient = {resourceType: 'Patient', id: 'synthetic-patient'};
    spyOn(TestBed.inject(FastenApiService), 'importSandboxPatient').and.returnValues(
      throwError(new Error('synthetic failure')), of({source: new Source({id: 'imported-source'})}));
    await component.importEpicSandboxPatient();
    fixture.detectChanges();
    expect(fixture.nativeElement.textContent).toContain('retry without signing in again');
    expect(component.sandboxPatient).toBeDefined();
    await component.importEpicSandboxPatient();
    fixture.detectChanges();
    expect(fixture.nativeElement.textContent).toContain('View imported Patient');
    expect(component.sandboxPatient).toBeUndefined();
  });

  it('cancels a pending connection when the page is destroyed', async () => {
    let signal: AbortSignal;
    spyOn(TestBed.inject(EpicSandboxService), 'connect').and.callFake(abort => {
      signal = abort;
      return new Promise((_resolve, reject) => abort.addEventListener('abort', () => reject(new Error('cancelled'))));
    });
    const pending = component.connectEpicSandbox();
    fixture.destroy();
    await pending;
    expect(signal.aborted).toBeTrue();
  });
});
