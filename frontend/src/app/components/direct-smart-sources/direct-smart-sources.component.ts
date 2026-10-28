import {Component, EventEmitter, OnDestroy, OnInit, Output} from '@angular/core';
import {Patient} from 'fhir/r4';
import {DirectSmartCleanupError, DirectSmartService} from '../../services/direct-smart.service';
import {FastenApiService} from '../../services/fasten-api.service';
import {SmartConnection, SmartProvider} from '../../models/direct-smart';
import {Source} from '../../models/fasten/source';

@Component({selector: 'app-direct-smart-sources', templateUrl: './direct-smart-sources.component.html'})
export class DirectSmartSourcesComponent implements OnInit, OnDestroy {
  @Output() imported = new EventEmitter<void>();
  providers: SmartProvider[] = [];
  selectedProviderID = '';
  loading = true;
  error = '';
  status = '';
  controller?: AbortController;
  patient?: Patient;
  connection?: SmartConnection;
  activeProvider?: SmartProvider;
  importedSource?: Source;
  importing = false;
  disconnecting = false;
  private destroyed = false;

  constructor(private direct: DirectSmartService, private api: FastenApiService) {}

  ngOnInit(): void { void this.loadProviders(); }

  async loadProviders(): Promise<void> {
    if (this.controller || this.importing || this.disconnecting || this.patient) return;
    this.loading = true;
    this.error = '';
    try {
      const providers = await this.direct.providers();
      if (!this.destroyed) { this.providers = providers; this.selectedProviderID = providers[0]?.id || ''; }
    } catch {
      if (!this.destroyed) this.error = 'Could not load direct providers. Check this installation’s configuration and try again.';
    } finally { if (!this.destroyed) this.loading = false; }
  }

  async connect(): Promise<void> {
    const provider = this.providers.find(item => item.id === this.selectedProviderID);
    if (!provider || this.controller || this.importing || this.disconnecting || this.patient || this.destroyed) return;
    this.activeProvider = provider;
    this.importedSource = undefined;
    this.error = '';
    this.status = 'Your system browser will open. Sign in with the selected provider to retrieve a Patient preview.';
    const controller = this.controller = new AbortController();
    try {
      const result = await this.direct.connect(provider.id, controller.signal);
      if (this.destroyed || controller.signal.aborted) { await this.direct.disconnect(result.connection.id); return; }
      this.connection = result.connection;
      this.patient = result.patient;
      this.status = 'Patient retrieved. Review the preview before importing. Nothing has been saved yet.';
    } catch (error: unknown) {
      if (!this.destroyed) {
        this.status = controller.signal.aborted ? 'Connection cancelled.' : '';
        if (error instanceof DirectSmartCleanupError) this.error = error.message;
        else if (!controller.signal.aborted) this.error = 'The direct connection failed. Reconnect to try again.';
      }
    } finally { if (this.controller === controller) this.controller = undefined; }
  }

  cancel(): void { this.controller?.abort(); this.status = 'Cancelling connection…'; }

  async disconnect(): Promise<void> {
    if (this.importing || this.disconnecting) return;
    this.disconnecting = true;
    this.status = 'Discarding preview and releasing the direct session…';
    this.controller?.abort();
    this.patient = undefined;
    const connection = this.connection;
    this.connection = undefined;
    try {
      if (connection) await this.direct.disconnect(connection.id);
      if (!this.destroyed) this.status = 'Preview discarded. No records were imported.';
    } catch {
      if (!this.destroyed) {
        this.status = '';
        this.error = 'Could not release the direct session. Close Chartrove to clear it.';
      }
    }
    finally { this.disconnecting = false; }
  }

  async importPatient(): Promise<void> {
    const patient = this.patient;
    const provider = this.activeProvider;
    if (!patient || !provider || this.importing || this.destroyed) return;
    this.importing = true;
    this.error = '';
    try {
      const result = await this.api.importSmartPatient(patient, {fhir_base_url: provider.fhir_base_url, display: provider.display}).toPromise();
      if (!this.destroyed) {
        this.importedSource = result.source;
        this.patient = undefined;
        this.status = result.alreadyImported ? 'This Patient snapshot is already imported. No existing records were changed.' : 'Patient snapshot imported. Automatic synchronization is not enabled.';
        this.imported.emit();
      }
      const connection = this.connection;
      this.connection = undefined;
      if (connection) {
        try { await this.direct.disconnect(connection.id); }
        catch { if (!this.destroyed) this.error = 'Import completed, but the direct session could not be released. Close Chartrove to clear it.'; }
      }
    } catch {
      if (!this.destroyed) this.error = 'The local import failed. You can retry without signing in again.';
    } finally { this.importing = false; }
  }

  get patientName(): string {
    const names = this.patient?.name;
    if (!Array.isArray(names) || !names[0] || typeof names[0] !== 'object') return 'Name not provided';
    const name = names[0];
    if (typeof name.text === 'string' && name.text) return name.text;
    const given = Array.isArray(name.given) ? name.given.filter(item => typeof item === 'string') : [];
    return [...given, ...(typeof name.family === 'string' ? [name.family] : [])].join(' ') || 'Name not provided';
  }

  get birthDate(): string { return typeof this.patient?.birthDate === 'string' ? this.patient.birthDate : 'Not provided'; }
  get gender(): string { return typeof this.patient?.gender === 'string' ? this.patient.gender : 'Not provided'; }

  ngOnDestroy(): void {
    this.destroyed = true;
    this.controller?.abort();
    this.patient = undefined;
    const connection = this.connection;
    this.connection = undefined;
    // A quitting/unavailable host already clears its in-memory credentials.
    // Handle teardown rejection without logging Patient or provider payloads.
    if (connection) void this.direct.disconnect(connection.id).catch(() => { this.error = 'Close Chartrove to clear the direct session.'; });
  }
}
