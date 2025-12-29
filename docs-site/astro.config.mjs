import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import tailwind from '@astrojs/tailwind';
import { readFileSync } from 'fs';

const conductorGrammar = JSON.parse(
  readFileSync('./grammars/conductor.tmLanguage.json', 'utf-8')
);

export default defineConfig({
  site: 'https://tombee.github.io',
  base: '/conductor',
  integrations: [
    tailwind({
      // Don't inject base styles - we handle this in our landing page
      applyBaseStyles: false,
    }),
    starlight({
      title: 'Conductor',
      description: 'AI workflows as simple as shell scripts',
      social: {
        github: 'https://github.com/tombee/conductor',
      },
      editLink: {
        baseUrl: 'https://github.com/tombee/conductor/edit/main/docs/',
      },
      customCss: ['./src/styles/custom.css'],
      expressiveCode: {
        shiki: {
          langs: [conductorGrammar],
        },
      },
      sidebar: [
        { label: 'Home', slug: 'index' },
        {
          label: 'Getting Started',
          items: [
            { label: 'Overview', slug: 'getting-started' },
            { label: 'First Workflow', slug: 'getting-started/first-workflow' },
          ],
        },
        {
          label: 'Building Workflows',
          items: [
            { label: 'Patterns', slug: 'building-workflows/patterns' },
            { label: 'Flow Control', slug: 'building-workflows/flow-control' },
            { label: 'Error Handling', slug: 'building-workflows/error-handling' },
            { label: 'Testing', slug: 'building-workflows/testing' },
            { label: 'Debugging', slug: 'building-workflows/debugging' },
            { label: 'Performance', slug: 'building-workflows/performance' },
            { label: 'Profiles', slug: 'building-workflows/profiles' },
            { label: 'Controller', slug: 'building-workflows/controller' },
            { label: 'Endpoints', slug: 'building-workflows/endpoints' },
          ],
        },
        {
          label: 'Examples',
          collapsed: true,
          autogenerate: { directory: 'examples' },
        },
        {
          label: 'Reference',
          items: [
            { label: 'CLI', slug: 'reference/cli' },
            { label: 'Workflow Schema', slug: 'reference/workflow-schema' },
            { label: 'Configuration', slug: 'reference/configuration' },
            { label: 'Cheatsheet', slug: 'reference/cheatsheet' },
            { label: 'API', slug: 'reference/api' },
            { label: 'Error Codes', slug: 'reference/error-codes' },
          ],
        },
        {
          label: 'Actions',
          items: [
            { label: 'File', slug: 'reference/actions/file' },
            { label: 'Shell', slug: 'reference/actions/shell' },
            { label: 'HTTP', slug: 'reference/actions/http' },
            { label: 'Transform', slug: 'reference/actions/transform' },
          ],
        },
        {
          label: 'Integrations',
          items: [
            { label: 'GitHub', slug: 'reference/integrations/github' },
            { label: 'Slack', slug: 'reference/integrations/slack' },
            { label: 'Discord', slug: 'reference/integrations/discord' },
            { label: 'Jira', slug: 'reference/integrations/jira' },
            { label: 'Jenkins', slug: 'reference/integrations/jenkins' },
            { label: 'Custom', slug: 'reference/integrations/custom' },
            { label: 'Runbooks', slug: 'reference/integrations/runbooks' },
          ],
        },
        {
          label: 'Production',
          collapsed: false,
          items: [
            { label: 'Deployment', slug: 'production/deployment' },
            { label: 'Security', slug: 'production/security' },
            { label: 'Monitoring', slug: 'production/monitoring' },
            { label: 'Startup', slug: 'production/startup' },
            { label: 'Troubleshooting', slug: 'production/troubleshooting' },
          ],
        },
        {
          label: 'Contributing',
          collapsed: false,
          items: [
            { label: 'Overview', slug: 'contributing' },
            { label: 'Development Setup', slug: 'contributing/development-setup' },
            { label: 'Custom Tools', slug: 'contributing/custom-tools' },
            { label: 'Embedding', slug: 'contributing/embedding' },
          ],
        },
        { label: 'FAQ', slug: 'faq' },
        {
          label: 'Architecture',
          collapsed: true,
          autogenerate: { directory: 'architecture' },
        },
      ],
    }),
  ],
});
