// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
import { getAdvisory } from "$lib/api/advisory_detail";
import type { PageLoad } from "./$types";

export const load: PageLoad = async ({ params }) => {
  const advisory = await getAdvisory(params.id);
  return { advisory };
};
