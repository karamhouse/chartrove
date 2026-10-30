// This file is required by karma.conf.js and loads recursively all the .spec and framework files

import 'zone.js/testing';
import { getTestBed } from '@angular/core/testing';
import { NgModule, provideZoneChangeDetection } from '@angular/core';
import {
  BrowserDynamicTestingModule,
  platformBrowserDynamicTesting
} from '@angular/platform-browser-dynamic/testing';


// Match the application's explicit Zone.js change detection in main.ts.
@NgModule({providers: [provideZoneChangeDetection()]})
class ZoneTestingModule {}

// First, initialize the Angular testing environment.
getTestBed().initTestEnvironment(
  [BrowserDynamicTestingModule, ZoneTestingModule],
  platformBrowserDynamicTesting()
);
