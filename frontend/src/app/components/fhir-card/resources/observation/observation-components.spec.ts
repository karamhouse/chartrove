import {TestBed} from '@angular/core/testing';
import {provideRouter} from '@angular/router';
import {ObservationComponent} from './observation.component';
import {ObservationModel} from '../../../../../lib/models/resources/observation-model';

// Dropping component values, merging repeated codes, replacing false/zero,
// borrowing the parent reference range or mutating source data must fail here.
describe('Observation component values in the clinical card', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({imports: [ObservationComponent], providers: [provideRouter([])]}).compileComponents();
  });

  function render(components: object[], extra: object = {}) {
    const raw = {resourceType: 'Observation', id: 'synthetic-panel', status: 'final',
      code: {text: 'Synthetic panel'}, subject: {reference: 'Patient/synthetic-patient'},
      effectiveDateTime: '2020-01-01', component: components, ...extra};
    const before = JSON.stringify(raw);
    const model = new ObservationModel(raw);
    model.source_id = 'synthetic-source';
    model.source_resource_id = 'synthetic-panel';
    const fixture = TestBed.createComponent(ObservationComponent);
    fixture.componentInstance.displayModel = model;
    fixture.detectChanges();
    expect(JSON.stringify(raw)).toBe(before);
    return {element: fixture.nativeElement as HTMLElement, model, raw, before};
  }

  it('shows each pressure measurement with its own supplied code and units', () => {
    const {element} = render([
      {code: {coding: [{system: 'http://loinc.org', code: '8480-6', display: 'Systolic'}]}, valueQuantity: {value: 120, unit: 'mmHg'}},
      {code: {coding: [{system: 'http://loinc.org', code: '8462-4', display: 'Diastolic'}]}, valueQuantity: {value: 80, unit: 'mmHg'}},
    ]);
    for (const text of ['Systolic', '8480-6', '120 mmHg', 'Diastolic', '8462-4', '80 mmHg']) {
      expect(element.textContent).withContext(text).toContain(text);
    }
  });

  it('preserves quantity zero, integer zero, false and an explicit empty string', () => {
    const {element} = render([
      {code: {text: 'Zero quantity'}, valueQuantity: {value: 0, unit: '%'}},
      {code: {text: 'Zero integer'}, valueInteger: 0},
      {code: {text: 'False assertion'}, valueBoolean: false},
      {code: {text: 'Empty text'}, valueString: ''},
    ]);
    const values = Array.from(element.querySelectorAll('tr')).filter(row => row.cells[0].textContent.trim() === 'Value').map(row => row.cells[1].textContent.trim());
    expect(values).toEqual(['0 %', '0', 'false', '']);
  });

  it('keeps Unicode and coded text as escaped source assertions', () => {
    const {element} = render([{code: {text: 'Café 零 صفر'}, valueCodeableConcept: {text: '<img src=x onerror=alert(1)> Negative'}}]);
    expect(element.textContent).toContain('Café 零 صفر');
    expect(element.textContent).toContain('<img src=x onerror=alert(1)> Negative');
    expect(element.querySelector('img')).toBeNull();
  });

  it('shows a supplied absence reason without inventing a numeric reading', () => {
    const {element} = render([{code: {text: 'Unavailable measurement'}, dataAbsentReason: {text: 'Not performed'}}]);
    expect(element.textContent).toContain('Unavailable measurement');
    expect(element.textContent).toContain('Not performed (data absent)');
    expect(element.querySelector('observation-visualization')).toBeNull();
  });

  it('retains repeated component codes and separate reference ranges in order', () => {
    const {element} = render([
      {code: {text: 'Repeated'}, valueQuantity: {value: 3, unit: 'mg'}, referenceRange: [{low: {value: 0, unit: 'mg'}, high: {value: 5, unit: 'mg'}}]},
      {code: {text: 'Repeated'}, valueQuantity: {value: 7, unit: 'g'}, referenceRange: [{low: {value: 6, unit: 'g'}, high: {value: 8, unit: 'g'}}]},
    ]);
    const text = element.textContent;
    expect(text).toContain('3 mg'); expect(text).toContain('7 g');
    expect(text).toContain('0 mg – 5 mg'); expect(text).toContain('6 g – 8 g');
    expect(text.indexOf('3 mg')).toBeLessThan(text.indexOf('7 g'));
    expect(text.match(/Repeated/g)?.length).toBe(2);
  });

  it('retains the parent reading, dates, record link and original resource', () => {
    const {element, model, raw, before} = render([{code: {text: 'Part'}, valueString: 'Component assertion'}], {valueString: 'Parent assertion', issued: '2020-01-02'});
    for (const text of ['Parent assertion', 'Component assertion', '2020-01-01', '2020-01-02']) expect(element.textContent).toContain(text);
    expect(element.querySelector('.card-footer a')?.getAttribute('href')).toBe('/explore/synthetic-source/resource/synthetic-panel');
    expect(model.fhirResource).toBe(raw);
    expect(JSON.stringify(raw)).toBe(before);
  });

  it('keeps a scalar observation unchanged when no component is supplied', () => {
    const {element} = render([], {valueInteger: 9});
    const values = Array.from(element.querySelectorAll('tr')).filter(row => row.cells[0].textContent.trim() === 'Value').map(row => row.cells[1].textContent.trim());
    expect(values).toEqual(['9']);
  });
});
