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
        { label: 'Quick Start', slug: 'quick-start' },
        {
          label: 'Learn',
          items: [
            { label: 'What is Conductor?', slug: 'learn/overview' },
            { label: 'Installation', slug: 'learn/installation' },
            {
              label: 'Concepts',
              items: [
                { label: 'Workflows and Steps', slug: 'learn/concepts/workflows-steps' },
                { label: 'Inputs and Outputs', slug: 'learn/concepts/inputs-outputs' },
                { label: 'Template Variables', slug: 'learn/concepts/template-variables' },
              ],
            },
            {
              label: 'Tutorials',
              items: [
                { label: 'First Workflow', slug: 'learn/tutorials/first-workflow' },
                { label: 'Code Review Bot', slug: 'learn/tutorials/code-review-bot' },
                { label: 'Slack Integration', slug: 'learn/tutorials/slack-integration' },
                { label: 'Multi-Agent Workflows', slug: 'learn/tutorials/multi-agent-workflows' },
              ],
            },
          ],
        },
        {
          label: 'Guides',
          autogenerate: { directory: 'guides' },
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
          label: 'Deployment',
          items: [
            { label: 'Overview', slug: 'deployment' },
            {
              label: 'Platforms',
              items: [
                { label: 'exe.dev', slug: 'deployment/exe-dev' },
                { label: 'Kubernetes', slug: 'deployment/kubernetes' },
              ],
            },
            {
              label: 'Modes',
              items: [
                { label: 'Docker', slug: 'deployment/docker' },
                { label: 'Bare Metal', slug: 'deployment/bare-metal' },
              ],
            },
          ],
        },
        {
          label: 'Operations',
          collapsed: true,
          autogenerate: { directory: 'operations' },
        },
        {
          label: 'Extending',
          collapsed: true,
          autogenerate: { directory: 'extending' },
        },
      ],
    }),
  ],
});
