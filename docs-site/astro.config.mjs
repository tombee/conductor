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
        { label: 'Getting Started', slug: 'getting-started' },
        { label: 'Concepts', slug: 'concepts' },
        {
          label: 'Tutorial',
          items: [
            { label: 'Overview', slug: 'tutorial' },
            { label: '1. First Recipe', slug: 'tutorial/01-first-recipe' },
            { label: '2. Better Recipe', slug: 'tutorial/02-better-recipe' },
            { label: '3. Meal Plan', slug: 'tutorial/03-meal-plan' },
            { label: '4. Pantry Check', slug: 'tutorial/04-pantry-check' },
            { label: '5. Weekly Plan', slug: 'tutorial/05-weekly-plan' },
            { label: '6. Save to Notion', slug: 'tutorial/06-save-to-notion' },
            { label: '7. Deploy', slug: 'tutorial/07-deploy' },
          ],
        },
        {
          label: 'Features',
          items: [
            { label: 'Inputs & Outputs', slug: 'features/inputs-outputs' },
            { label: 'Actions', slug: 'features/actions' },
            { label: 'Integrations', slug: 'features/integrations' },
            { label: 'Parallel Execution', slug: 'features/parallel' },
            { label: 'Loops', slug: 'features/loops' },
            { label: 'Conditions', slug: 'features/conditions' },
            { label: 'Triggers', slug: 'features/triggers' },
            { label: 'Model Tiers', slug: 'features/model-tiers' },
            { label: 'MCP Servers', slug: 'features/mcp' },
          ],
        },
      ],
    }),
  ],
});
