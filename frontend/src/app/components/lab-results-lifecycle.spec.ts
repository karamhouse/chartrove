import {ComponentFixture, TestBed} from '@angular/core/testing';
import {NgbActiveModal} from '@ng-bootstrap/ng-bootstrap';
import {of, Subject} from 'rxjs';
import {MedicalRecordWizardAddLabResultsComponent} from './medical-record-wizard-add-lab-results/medical-record-wizard-add-lab-results.component';
import {MedicalRecordWizardEditLabResultsComponent} from './medical-record-wizard-edit-lab-results/medical-record-wizard-edit-lab-results.component';
import {NlmClinicalTableSearchService} from '../services/nlm-clinical-table-search.service';
import {FastenApiService} from '../services/fasten-api.service';
import {LabresultsQuestionnaire} from '../models/fasten/labresults-questionnaire';

const variants = [
  {component: MedicalRecordWizardAddLabResultsComponent, form: 'newLabPanelTypeaheadForm'},
  {component: MedicalRecordWizardEditLabResultsComponent, form: 'labPanelTypeaheadForm'},
];

for (const variant of variants) {
  describe(`${variant.component.name} widget lifecycle`, () => {
    let fixture: ComponentFixture<any>;
    let component: MedicalRecordWizardAddLabResultsComponent | MedicalRecordWizardEditLabResultsComponent;
    let modal: jasmine.SpyObj<NgbActiveModal>;
    let formResponse: Subject<LabresultsQuestionnaire>;
    let util: {checkValidity: jasmine.Spy; getFormFHIRData: jasmine.Spy};
    let originalLForms: PropertyDescriptor | undefined;

    function questionnaire(): LabresultsQuestionnaire {
      return {
        type: 'LOINC', code: 'synthetic-panel', name: 'Synthetic panel',
        codingInstructions: null, copyrightNotice: null, items: [],
        lformsVersion: '34.0.0', codeSystem: 'http://loinc.org',
      };
    }

    function button(): HTMLButtonElement {
      return fixture.nativeElement.querySelector('.modal-footer button');
    }

    function readyWidget(): HTMLElement {
      component.questionnaire = questionnaire();
      fixture.detectChanges();
      const widget: HTMLElement = fixture.nativeElement.querySelector('wc-lhc-form');
      expect(button().disabled).toBeTrue();
      widget.dispatchEvent(new CustomEvent('onFormReady'));
      fixture.detectChanges();
      expect(button().disabled).toBeFalse();
      return widget;
    }

    beforeEach(async () => {
      originalLForms = Object.getOwnPropertyDescriptor(globalThis, 'LForms');
      util = {
        checkValidity: jasmine.createSpy('checkValidity').and.returnValue(null),
        getFormFHIRData: jasmine.createSpy('getFormFHIRData').and.returnValue({
          resourceType: 'Bundle', type: 'transaction', entry: [],
        }),
      };
      Object.defineProperty(globalThis, 'LForms', {configurable: true, value: {Util: util}});
      modal = jasmine.createSpyObj<NgbActiveModal>('modal', ['close', 'dismiss']);
      formResponse = new Subject<LabresultsQuestionnaire>();
      await TestBed.configureTestingModule({
        imports: [variant.component],
        providers: [
          {provide: NgbActiveModal, useValue: modal},
          {provide: FastenApiService, useValue: {}},
          {provide: NlmClinicalTableSearchService, useValue: {
            searchLabPanels: () => of([]),
            searchLabPanelQuestionnaire: () => formResponse,
          }},
        ],
      }).compileComponents();
      fixture = TestBed.createComponent<any>(variant.component);
      component = fixture.componentInstance;
      if (component instanceof MedicalRecordWizardEditLabResultsComponent) {
        component.observations = [];
      }
      fixture.detectChanges();
    });

    afterEach(() => {
      formResponse.complete();
      if (originalLForms) {
        Object.defineProperty(globalThis, 'LForms', originalLForms);
      } else {
        delete (globalThis as any).LForms;
      }
    });

    it('does not submit before a form is ready', () => {
      expect(button().disabled).toBeTrue();
      component.submit();
      expect(util.checkValidity).not.toHaveBeenCalled();
      expect(util.getFormFHIRData).not.toHaveBeenCalled();
      expect(modal.close).not.toHaveBeenCalled();
    });

    it('handles the real ready event and exports only its own widget', () => {
      const widget = readyWidget();
      button().click();
      expect(util.checkValidity).toHaveBeenCalledWith(widget.parentElement);
      expect(util.getFormFHIRData).toHaveBeenCalledWith(
        'DiagnosticReport', 'R4', widget.parentElement, {bundleType: 'transaction'});
      expect(modal.close).toHaveBeenCalledTimes(1);
    });

    it('keeps an invalid form open without exporting it', () => {
      readyWidget();
      util.checkValidity.and.returnValue(['Synthetic validation error']);
      button().click();
      expect(util.getFormFHIRData).not.toHaveBeenCalled();
      expect(modal.close).not.toHaveBeenCalled();
    });

    it('handles widget errors without exposing their details or allowing submission', () => {
      const widget = readyWidget();
      widget.dispatchEvent(new CustomEvent('onError', {detail: 'Synthetic internal detail'}));
      fixture.detectChanges();
      expect(button().disabled).toBeTrue();
      expect(fixture.nativeElement.querySelector('[role="alert"]').textContent)
        .toContain('Close this form and try again.');
      expect(fixture.nativeElement.textContent).not.toContain('Synthetic internal detail');
      component.submit();
      expect(modal.close).not.toHaveBeenCalled();
    });

    it('clears readiness when another panel loads and waits for its ready event', () => {
      readyWidget();
      fixture.componentInstance[variant.form].get('data').setValue({id: 'synthetic-next'});
      fixture.detectChanges();
      expect(button().disabled).toBeTrue();
      expect(fixture.nativeElement.querySelector('wc-lhc-form')).toBeNull();
      formResponse.next(questionnaire());
      fixture.detectChanges();
      expect(button().disabled).toBeTrue();
      fixture.nativeElement.querySelector('wc-lhc-form').dispatchEvent(new CustomEvent('onFormReady'));
      fixture.detectChanges();
      expect(button().disabled).toBeFalse();
    });

    it('shows a safe error when the panel request fails', () => {
      fixture.componentInstance[variant.form].get('data').setValue({id: 'synthetic-failure'});
      formResponse.error(new Error('Synthetic internal detail'));
      fixture.detectChanges();
      expect(component.loading).toBeFalse();
      expect(button().disabled).toBeTrue();
      expect(fixture.nativeElement.querySelector('[role="alert"]').textContent)
        .toContain('Close this form and try again.');
      expect(fixture.nativeElement.textContent).not.toContain('Synthetic internal detail');
    });

    it('keeps the modal open when export throws', () => {
      readyWidget();
      util.getFormFHIRData.and.throwError('Synthetic internal detail');
      button().click();
      fixture.detectChanges();
      expect(button().disabled).toBeTrue();
      expect(modal.close).not.toHaveBeenCalled();
      expect(fixture.nativeElement.querySelector('[role="alert"]').textContent)
        .toContain('Close this form and try again.');
      expect(fixture.nativeElement.textContent).not.toContain('Synthetic internal detail');
    });
  });
}
