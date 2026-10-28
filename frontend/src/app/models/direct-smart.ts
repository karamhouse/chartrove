export interface SmartProvider {
  readonly id: string;
  readonly display: string;
  readonly fhir_base_url: string;
}

export interface SmartConnection {
  readonly id: string;
  readonly provider_id: string;
  readonly patient_id: string;
  readonly expires_at: number;
}

export interface SmartBridge {
  providers(): Promise<SmartProvider[]>;
  connect(providerID: string): Promise<SmartConnection>;
  readPatient(connectionID: string): Promise<string>;
  cancel(providerID: string): Promise<void>;
  disconnect(connectionID: string): Promise<void>;
}

export interface SmartImportOrigin {
  readonly fhir_base_url: string;
  readonly display: string;
}
