// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  docs: [
    {
      type: 'doc',
      id: 'introduction',
      label: 'Introduction',
    },
    {
      type: 'category',
      label: 'Installation',
      items: [
        'installation/requirements',
        'installation/quickstart',
        'installation/rpm-packages',
        'installation/deb-packages',
        'installation/build-from-source',
      ],
    },
    {
      type: 'category',
      label: 'Onboarding Hosts',
      items: [
        'onboarding/overview',
        'onboarding/agent-mode',
        'onboarding/ssh-pull-mode',
        'onboarding/manual-mode',
      ],
    },
    {
      type: 'category',
      label: 'Guides',
      items: [
        'guides/managing-hosts',
        'guides/vulnerability-matching',
        'guides/advisory-sync',
        'guides/scope-mappings',
        'guides/ssl-tls',
        'guides/notifications',
      ],
    },
    {
      type: 'category',
      label: 'Reference',
      items: [
        {
          type: 'category',
          label: 'Configuration',
          items: [
            'configuration/server',
            'configuration/agent',
          ],
        },
        {
          type: 'category',
          label: 'CLI',
          items: [
            'cli/server',
            'cli/agent',
          ],
        },
        {
          type: 'category',
          label: 'API',
          items: [
            'api/overview',
            'api/authentication',
            'api/hosts',
            'api/advisories',
            'api/websocket',
          ],
        },
      ],
    },
    {
      type: 'category',
      label: 'Architecture',
      items: [
        'architecture/overview',
        'architecture/agent-collector',
        'architecture/advisory-database',
        'architecture/matching-engine',
      ],
    },
    {
      type: 'category',
      label: 'Development',
      items: [
        'development/building',
        'development/dev-setup',
        'development/migrations',
        'development/sqlc',
        'development/testing',
        'development/protos',
        'development/frontend',
      ],
    },
    'contributing',
    'faq',
  ],
};

export default sidebars;