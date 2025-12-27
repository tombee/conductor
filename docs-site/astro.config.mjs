import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://tombee.github.io',
  base: '/conductor',
  integrations: [
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
      sidebar: [
        { label: 'Home', slug: 'index' },
        {
          label: 'Getting Started',
          items: [
            { label: 'Overview', slug: 'getting-started' },
            { label: 'Installation', slug: 'getting-started/installation' },
            { label: 'First Workflow', slug: 'getting-started/first-workflow' },
            { label: 'Core Concepts', slug: 'getting-started/concepts' },
          ],
        },
        {
          label: 'Building Workflows',
          items: [
            { label: 'Overview', slug: 'building-workflows' },
            { label: 'Flow Control', slug: 'building-workflows/flow-control' },
            { label: 'Error Handling', slug: 'building-workflows/error-handling' },
            { label: 'Testing', slug: 'building-workflows/testing' },
            { label: 'Debugging', slug: 'building-workflows/debugging' },
            { label: 'Performance', slug: 'building-workflows/performance' },
            { label: 'Profiles', slug: 'building-workflows/profiles' },
            { label: 'Daemon Mode', slug: 'building-workflows/daemon-mode' },
          ],
        },
        {
          label: 'Examples',
          items: [
            { label: 'Overview', slug: 'examples' },
            { label: 'Code Review', slug: 'examples/code-review' },
            { label: 'Issue Triage', slug: 'examples/issue-triage' },
            { label: 'Slack Integration', slug: 'examples/slack-integration' },
            { label: 'IaC Review', slug: 'examples/iac-review' },
            { label: 'Write Song', slug: 'examples/write-song' },
          ],
        },
        {
          label: 'Reference',
          items: [
            { label: 'CLI', slug: 'reference/cli' },
            { label: 'Workflow Schema', slug: 'reference/workflow-schema' },
            { label: 'Configuration', slug: 'reference/configuration' },
            { label: 'API', slug: 'reference/api' },
            { label: 'Error Codes', slug: 'reference/error-codes' },
            {
              label: 'Operations',
              items: [
                { label: 'File', slug: 'reference/connectors/file' },
                { label: 'Shell', slug: 'reference/connectors/shell' },
                { label: 'HTTP', slug: 'reference/connectors/http' },
                { label: 'Transform', slug: 'reference/connectors/transform' },
              ],
            },
            {
              label: 'Service Integrations',
              items: [
                { label: 'GitHub', slug: 'reference/connectors/github' },
                { label: 'Slack', slug: 'reference/connectors/slack' },
                { label: 'Discord', slug: 'reference/connectors/discord' },
                { label: 'Jira', slug: 'reference/connectors/jira' },
                { label: 'Jenkins', slug: 'reference/connectors/jenkins' },
                { label: 'Custom', slug: 'reference/connectors/custom' },
                { label: 'Runbooks', slug: 'reference/connectors/runbooks' },
              ],
            },
          ],
        },
        {
          label: 'Production',
          collapsed: false,
          items: [
            { label: 'Overview', slug: 'production' },
            {
              label: 'Deployment',
              items: [
                { label: 'Overview', slug: 'production/deployment' },
                { label: 'exe.dev', slug: 'production/deployment/exe-dev' },
                { label: 'Docker', slug: 'production/deployment/docker' },
                { label: 'Kubernetes', slug: 'production/deployment/kubernetes' },
                { label: 'Bare Metal', slug: 'production/deployment/bare-metal' },
              ],
            },
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
        {
          label: 'Architecture',
          collapsed: true,
          autogenerate: { directory: 'architecture' },
        },
      ],
    }),
  ],
});
