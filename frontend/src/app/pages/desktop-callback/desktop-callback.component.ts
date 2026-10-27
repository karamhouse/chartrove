import { Component, OnInit } from '@angular/core';
import {ActivatedRoute} from '@angular/router';
import {GetDesktopBridge} from '../../../lib/utils/desktop_bridge';

export function readDesktopAuthCallback(state: string, href: string): Record<string, string> {
  if (!/^[A-Za-z0-9_-]{1,128}$/.test(state || '')) throw new Error('Invalid callback state');
  const url = new URL(href);
  const fragment = new URLSearchParams(url.hash.split('?')[1] || '');
  const data: Record<string, string> = {state};
  for (const key of ['state', 'code', 'error']) {
    const values = [...url.searchParams.getAll(key), ...fragment.getAll(key)];
    if (values.length > 1) throw new Error('Duplicate callback parameter');
    if (values.length) data[key] = values[0];
  }
  if (data['state'] !== state) throw new Error('Callback state mismatch');
  if ((!data['code'] && !data['error']) || (data['code'] && data['error'])) throw new Error('Invalid callback response');
  return data;
}

@Component({
  selector: 'app-desktop-callback',
  templateUrl: './desktop-callback.component.html',
  styleUrls: ['./desktop-callback.component.scss']
})
export class DesktopCallbackComponent implements OnInit {

  //This component is used to redirect the user to the desktop app after they have authenticated with a source
  constructor(private activatedRoute : ActivatedRoute) { }

  ngOnInit(): void {
    const state = this.activatedRoute.snapshot.paramMap.get('state');
    if (!state) return;
    let data: Record<string, string>;
    try { data = readDesktopAuthCallback(state, window.location.href); }
    catch { data = {state, error: 'invalid_callback'}; }
    // OAuth codes belong to this one pending popup, not its URL history or logs.
    const clean = new URL(window.location.href);
    clean.search = '';
    clean.hash = clean.hash.split('?')[0];
    window.history.replaceState(window.history.state, '', clean.toString());
    GetDesktopBridge().then(bridge => bridge.completeAuth(data)).catch(() => {});
  }
}
