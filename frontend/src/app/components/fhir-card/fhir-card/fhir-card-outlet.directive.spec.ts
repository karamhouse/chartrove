import {Component, Input, ViewChild} from '@angular/core';
import {TestBed} from '@angular/core/testing';
import {FhirCardOutletDirective} from './fhir-card-outlet.directive';

@Component({template: '<ng-template fhirCardOutlet></ng-template>', standalone: false})
class OutletHost {
  @ViewChild(FhirCardOutletDirective, {static: true}) outlet: FhirCardOutletDirective;
}

@Component({template: '<p>{{label}}</p>'})
class SyntheticCard {
  @Input() label = '';
}

describe('FhirCardOutletDirective', () => {
  it('renders a component class and destroys it when the outlet is cleared', async () => {
    await TestBed.configureTestingModule({
      declarations: [OutletHost, FhirCardOutletDirective],
      imports: [SyntheticCard],
    }).compileComponents();
    const fixture = TestBed.createComponent(OutletHost);
    fixture.detectChanges();
    const container = fixture.componentInstance.outlet.viewContainerRef;
    const card = container.createComponent(SyntheticCard);
    const destroyed = jasmine.createSpy('destroyed');
    card.onDestroy(destroyed);
    card.setInput('label', 'Synthetic FHIR card');
    card.changeDetectorRef.detectChanges();
    expect(fixture.nativeElement.textContent).toContain('Synthetic FHIR card');
    expect(container.length).toBe(1);

    container.clear();
    expect(container.length).toBe(0);
    expect(fixture.nativeElement.textContent).not.toContain('Synthetic FHIR card');
    expect(destroyed).toHaveBeenCalledTimes(1);
  });
});
