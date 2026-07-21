// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const REPO_URL = 'https://github.com/patchbase/patchbase';

const config: Config = {
  title: 'PatchBase',
  tagline: 'Self-hosted vulnerability and patch management for Linux servers',
  favicon: 'img/favicon.ico',

  url: 'https://docs.patchbase.net',
  baseUrl: '/',

  organizationName: 'patchbase',
  projectName: 'patchbase',

  onBrokenLinks: 'throw',

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  presets: [
    [
      'classic',
      {
        docs: {
          path: '.',
          routeBasePath: '/',
          sidebarPath: './src/sidebars.ts',
          editUrl: `${REPO_URL}/tree/main/docs/`,
          include: ['**/*.{md,mdx}'],
          exclude: [
            'node_modules/**',
            'build/**',
            'src/**',
            'docusaurus.config.ts',
            'tsconfig.json',
          ],
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.ClassicPresetOptions,
    ],
  ],

  markdown: {
    mermaid: true,
  },
  themes: ['@docusaurus/theme-mermaid'],

  themeConfig: {
    colorMode: {
      defaultMode: 'dark',
      disableSwitch: false,
      respectPrefersColorScheme: true,
    },
    navbar: {
      title: 'PatchBase',
      logo: {
        alt: 'PatchBase logo',
        src: 'img/logo.png',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'docs',
          position: 'left',
          label: 'Docs',
        },
        {
          href: REPO_URL,
          position: 'right',
          className: 'header-github-link',
          'aria-label': 'GitHub repository',
        },
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Documentation',
          items: [
            {
              label: 'Introduction',
              to: '/',
            },
            {
              label: 'Installation',
              to: '/installation/requirements',
            },
            {
              label: 'Onboarding Hosts',
              to: '/onboarding/overview',
            },
          ],
        },
        {
          title: 'Reference',
          items: [
            {
              label: 'Configuration',
              to: '/configuration/server',
            },
            {
              label: 'CLI',
              to: '/cli/server',
            },
            {
              label: 'API',
              to: '/api/overview',
            },
          ],
        },
        {
          title: 'Community',
          items: [
            {
              label: 'GitHub',
              href: REPO_URL,
            },
            {
              label: 'Contributing',
              to: '/contributing',
            },
          ],
        },
      ],
      copyright: `Copyright &copy; ${new Date().getFullYear()} PatchBase. Built with Docusaurus.`,
    },
    prism: {
      theme: prismThemes.githubDark,
      darkTheme: prismThemes.githubDark,
      additionalLanguages: ['bash', 'yaml', 'json', 'go', 'sql'],
    },
  } satisfies Preset.ThemeConfig,
};

export default config;