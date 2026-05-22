export interface Host {
  id: string;
  onboarding_mode?: string;
  approval_status?: string;
  display_name: string | null;
  hostname: string;
  ip_address?: string;
  os_family: string;
  os_name: string;
  os_major: number;
  os_version: string;
  architecture: string;
  status: string;
  last_seen_at: string | null;
  overall_action: string;
  critical_count: number;
  important_count: number;
  moderate_count: number;
  actionable_count: number;
  available_updates: number;
  needs_reboot: number;
  needs_restart: number;
  no_fix: number;
  unknown: number;
  last_advisory_check_at?: string | null;
  state_updated_at?: string | null;
  pull_last_run_at?: string | null;
  pull_last_run_status?: string;
  pull_last_run_error?: string;
  created_at?: string;
  updated_at: string;
}

export interface HostSnapshot {
  id: string;
  host_id: string;
  collected_at: string;
  os_name: string;
  os_version: string;
  running_kernel_nevra: string;
  boot_time: string;
  has_process_data: boolean;
}

export interface Advisory {
  id: string;
  source_system: string;
  raw_source_id: string;
  source_url: string | null;
  vendor: string;
  advisory_type: string;
  severity: string;
  summary: string;
  published_at: string;
  updated_at: string;
  package_count: number;
  evidence_tier: string;
  is_security: boolean;
  product_streams: string[];
}

export interface DecisionRecord {
  package_name: string;
  installed_nevra: string | null;
  fixed_nevra: string | null;
  status: string;
  action: string;
  severity: string;
  reason_code: string;
  reason_text: string;
  advisory_id: string;
  advisory_raw_id: string;
  advisory_summary: string;
  advisory_severity: string;
}

export interface DecisionGroup {
  family: string;
  severity: string;
  action: string;
  advisory_count: number;
  package_count: number;
  updated_at: string;
  decisions: DecisionRecord[];
}

export interface ProductStream {
  id: string;
  vendor: string;
  distro_name: string;
  major_version: number;
  minor_version: number | null;
  architecture: string;
  repo_family: string;
  cpe: string | null;
  status: string;
  advisory_count: number;
}

export interface SourceSync {
  name: string;
  advisory_count: number;
  stream_count: number;
  last_sync: string | null;
  status: string;
}

export interface HostPullJob {
  id: string;
  host_id: string;
  status: string;
  started_at: string;
  completed_at: string | null;
  error: string | null;
}
