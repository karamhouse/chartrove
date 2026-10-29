import { Type } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { ArcElement, Chart } from 'chart.js';
import { of } from 'rxjs';
import { DashboardWidgetConfig } from '../models/widget/dashboard-widget-config';
import { FastenApiService } from '../services/fasten-api.service';
import { DashboardWidgetComponentInterface } from './dashboard-widget-component-interface';
import { DonutChartWidgetComponent } from './donut-chart-widget/donut-chart-widget.component';
import { DualGaugesWidgetComponent } from './dual-gauges-widget/dual-gauges-widget.component';
import { GroupedBarChartWidgetComponent } from './grouped-bar-chart-widget/grouped-bar-chart-widget.component';
import { SimpleLineChartWidgetComponent } from './simple-line-chart-widget/simple-line-chart-widget.component';

describe('Populated dashboard charts', () => {
  const widgets: Array<{
    name: string;
    component: Type<DashboardWidgetComponentInterface>;
    chartType: 'doughnut' | 'line' | 'bar';
    queryCount: number;
    canvasCount: number;
  }> = [
    { name: 'donut', component: DonutChartWidgetComponent, chartType: 'doughnut', queryCount: 1, canvasCount: 1 },
    { name: 'dual gauges', component: DualGaugesWidgetComponent, chartType: 'doughnut', queryCount: 2, canvasCount: 2 },
    { name: 'line', component: SimpleLineChartWidgetComponent, chartType: 'line', queryCount: 1, canvasCount: 1 },
    { name: 'grouped bar', component: GroupedBarChartWidgetComponent, chartType: 'bar', queryCount: 2, canvasCount: 1 },
  ];

  for (const widget of widgets) {
    it(`renders and disposes a populated ${widget.name} chart`, async () => {
      const api = jasmine.createSpyObj('FastenApiService', ['queryResources']);
      api.queryResources.and.returnValue(of({
        success: true, data: [{ label: 'Synthetic A', value: 12 }, { label: 'Synthetic B', value: 8 }],
      }));
      await TestBed.configureTestingModule({
        imports: [widget.component],
        providers: [{ provide: FastenApiService, useValue: api }],
      }).compileComponents();

      const fixture = TestBed.createComponent(widget.component);
      const config = new DashboardWidgetConfig();
      config.title_text = 'Synthetic chart';
      config.parsing = { label: 'label', key: 'value', xAxisKey: 'label', yAxisKey: 'value' };
      config.queries = Array.from({ length: widget.queryCount }, () => ({
        q: { select: [], from: 'Observation', where: {}, aggregations: { group_by: { field: 'code' } } },
      }));
      fixture.componentInstance.widgetConfig = config;
      fixture.detectChanges();

      const canvases: HTMLCanvasElement[] = Array.from(fixture.nativeElement.querySelectorAll('canvas'));
      expect(canvases.length).toBe(widget.canvasCount);
      expect(api.queryResources).toHaveBeenCalledTimes(widget.queryCount);
      for (const canvas of canvases) {
        const chart = Chart.getChart(canvas);
        expect(chart).toBeDefined();
        expect(chart.width).toBeGreaterThan(0);
        chart.update('none');
        const meta = chart.getDatasetMeta(0);
        expect(meta.type).toBe(widget.chartType);
        expect(meta.data.length).toBe(2);
        if (widget.chartType === 'doughnut') {
          const arc = meta.data[0];
          expect(arc instanceof ArcElement).toBeTrue();
          if (arc instanceof ArcElement) {
            expect(arc.circumference).toBeCloseTo(2 * Math.PI * 12 / 20, 5);
          }
        } else {
          expect(meta.data[0].y).toBeLessThan(meta.data[1].y);
        }
      }

      fixture.destroy();
      for (const canvas of canvases) {
        expect(Chart.getChart(canvas)).toBeUndefined();
      }
    });
  }
});
