// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
import { authenticatedRequest } from "$lib/api/request.js";

export interface AdvisoryDetail {
  id: string;
  source_system: string;
  raw_source_id: string;
  source_url: string | null;
  vendor: string;
  advisory_type: string;
  severity: string | null;
  summary: string | null;
  description: string | null;
  published_at: string | null;
  updated_at: string | null;
  evidence_tier: string;
  is_security: boolean;
}

export async function getAdvisory(id: string): Promise<AdvisoryDetail> {
  return authenticatedRequest(`/api/v1/advisories/${encodeURIComponent(id)}`);
}
