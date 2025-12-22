#!/usr/bin/env node
/**
 * Build-time doc processing: Copy docs/ to Starlight and convert tabs
 *
 * - Copies all .md files from docs/ preserving structure
 * - Converts MkDocs tabs (=== "Label") to Starlight <Tabs> components
 * - Creates index pages and homepage
 * - Outputs to src/content/docs/
 */

import { readFileSync, writeFileSync, mkdirSync, readdirSync, statSync, existsSync, rmSync } from 'fs';
import { join, dirname, basename, relative } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const DOCS_SRC = join(__dirname, '../../docs');
const DOCS_DEST = join(__dirname, '../src/content/docs');
const EXAMPLES_SRC = join(__dirname, '../../examples');

// Files/folders to skip
const SKIP = ['_templates', 'STYLE_GUIDE.md', '.DS_Store'];

// Example category mapping (example dir name -> docs category)
const EXAMPLE_CATEGORIES = {
  'code-review': 'code-review',
  'iac-review': 'devops',
  'security-audit': 'devops',
  'issue-triage': 'automation',
  'slack-integration': 'automation',
  'write-song': 'templates',
  'remote-workflows': 'templates',
};

// Only skip root index.md (we create a custom homepage)
const SKIP_ROOT_INDEX = true;

/**
 * Convert MkDocs tabs to Starlight Tabs component
 */
function convertTabs(content) {
  const lines = content.split('\n');
  const result = [];
  let hasTabs = false;
  let i = 0;

  while (i < lines.length) {
    const line = lines[i];
    const tabMatch = line.match(/^(\s*)===\s+"([^"]+)"/);

    if (tabMatch) {
      hasTabs = true;
      const indent = tabMatch[1] || '';
      const label = tabMatch[2];

      // Start Tabs if this is first tab in group
      const prevLine = result[result.length - 1] || '';
      if (!prevLine.includes('</TabItem>')) {
        result.push(`${indent}<Tabs>`);
      }

      result.push(`${indent}  <TabItem label="${label}">`);
      result.push('');

      const contentIndent = indent + '    ';
      i++;

      while (i < lines.length) {
        const contentLine = lines[i];
        if (contentLine.match(new RegExp(`^${indent}===\\s+"`))) break;
        if (contentLine.startsWith(contentIndent)) {
          result.push(indent + contentLine.slice(contentIndent.length));
          i++;
        } else if (contentLine.trim() === '') {
          const nextLine = lines[i + 1] || '';
          if (!nextLine.startsWith(contentIndent) && !nextLine.match(new RegExp(`^${indent}===\\s+"`))) break;
          result.push('');
          i++;
        } else {
          break;
        }
      }

      result.push(`${indent}  </TabItem>`);
      const nextLine = lines[i] || '';
      if (!nextLine.match(new RegExp(`^${indent}===\\s+"`))) {
        result.push(`${indent}</Tabs>`);
        result.push('');
      }
    } else {
      result.push(line);
      i++;
    }
  }

  return { content: result.join('\n'), hasTabs };
}

/**
 * Extract title from first H1
 */
function extractTitle(content) {
  const match = content.match(/^#\s+(.+)$/m);
  return match ? match[1].trim() : 'Untitled';
}

/**
 * Process an example file with workflow.yaml injection
 */
function processExampleFile(srcPath, destPath, workflowContent) {
  let content = readFileSync(srcPath, 'utf-8');
  const title = extractTitle(content);

  // Remove first H1 (title goes in frontmatter)
  content = content.replace(/^#\s+.+\n+/, '');

  // Remove MkDocs button classes (breaks MDX)
  content = content.replace(/\{\s*\.md-button[^}]*\}/g, '');

  // Inject workflow.yaml after the first paragraph if it exists
  if (workflowContent) {
    // Find first ## heading or end of first section
    const firstHeadingMatch = content.match(/\n##\s+/);
    const insertPos = firstHeadingMatch ? firstHeadingMatch.index : content.indexOf('\n\n', 50);

    if (insertPos > 0) {
      const workflowSection = `\n\n## Workflow Definition\n\n\`\`\`yaml\n${workflowContent.trim()}\n\`\`\`\n`;
      content = content.slice(0, insertPos) + workflowSection + content.slice(insertPos);
    }
  }

  // Fix internal links
  content = content.replace(/\]\(([^)#]+)(\.md)(#[^)]+)?\)/g, (match, path, ext, anchor) => {
    let cleanPath = path;
    if (cleanPath.endsWith('/index')) {
      cleanPath = cleanPath.replace(/\/index$/, '');
    } else if (cleanPath === 'index') {
      cleanPath = '.';
    }
    const finalPath = cleanPath + '/' + (anchor || '');
    return `](${finalPath.replace(/\/+$/, '/')})`;
  });

  // Convert tabs
  const { content: processed, hasTabs } = convertTabs(content);

  // Build frontmatter
  let final = `---\ntitle: "${title.replace(/"/g, '\\"')}"\n---\n\n`;
  if (hasTabs) {
    final += `import { Tabs, TabItem } from '@astrojs/starlight/components';\n\n`;
  }
  final += processed;

  // Determine output path
  mkdirSync(dirname(destPath), { recursive: true });
  if (hasTabs && !destPath.endsWith('.mdx')) {
    destPath = destPath.replace(/\.md$/, '.mdx');
  }

  writeFileSync(destPath, final);
  return { destPath, hasTabs };
}

/**
 * Process a single file
 */
function processFile(srcPath, destPath) {
  let content = readFileSync(srcPath, 'utf-8');
  const title = extractTitle(content);

  // Remove first H1 (title goes in frontmatter)
  content = content.replace(/^#\s+.+\n+/, '');

  // Remove MkDocs button classes (breaks MDX)
  content = content.replace(/\{\s*\.md-button[^}]*\}/g, '');

  // Fix internal links - convert relative .md links to work with Starlight
  // Note: We leave relative paths as-is since Starlight handles them correctly
  // Just need to convert .md extension and handle index.md specially
  content = content.replace(/\]\(([^)#]+)(\.md)(#[^)]+)?\)/g, (match, path, ext, anchor) => {
    // Remove .md extension, handle index.md -> directory path
    let cleanPath = path;
    if (cleanPath.endsWith('/index')) {
      cleanPath = cleanPath.replace(/\/index$/, '');
    } else if (cleanPath === 'index') {
      cleanPath = '.';
    }
    const finalPath = cleanPath + '/' + (anchor || '');
    return `](${finalPath.replace(/\/+$/, '/')})`;
  });

  // Convert tabs
  const { content: processed, hasTabs } = convertTabs(content);

  // Build frontmatter
  let final = `---\ntitle: "${title.replace(/"/g, '\\"')}"\n---\n\n`;
  if (hasTabs) {
    final += `import { Tabs, TabItem } from '@astrojs/starlight/components';\n\n`;
  }
  final += processed;

  // Determine output path
  mkdirSync(dirname(destPath), { recursive: true });
  if (hasTabs && !destPath.endsWith('.mdx')) {
    destPath = destPath.replace(/\.md$/, '.mdx');
  }

  writeFileSync(destPath, final);
  return { destPath, hasTabs };
}

/**
 * Recursively process directory
 */
function processDirectory(srcDir, destDir, isRoot = false) {
  const entries = readdirSync(srcDir);
  let count = 0;

  for (const entry of entries) {
    if (SKIP.includes(entry)) continue;
    // Skip only root index.md (we create a custom homepage)
    if (isRoot && SKIP_ROOT_INDEX && entry === 'index.md') continue;

    const srcPath = join(srcDir, entry);
    const destPath = join(destDir, entry);
    const stat = statSync(srcPath);

    if (stat.isDirectory()) {
      count += processDirectory(srcPath, destPath, false);
    } else if (entry.endsWith('.md')) {
      const { destPath: finalPath } = processFile(srcPath, destPath);
      const rel = relative(DOCS_DEST, finalPath);
      console.log(`✓ ${rel}`);
      count++;
    }
  }

  return count;
}

/**
 * Create Starlight homepage with components
 */
function createHomepage() {
  const content = `---
title: Conductor
description: A production-ready platform for AI agent workflows
---

import { Card, CardGrid, LinkCard } from '@astrojs/starlight/components';

Define AI agent workflows in YAML. Run them anywhere. Built-in observability, security, cost controls, and flexible deployment.

<CardGrid>
  <LinkCard title="What is Conductor?" description="Overview and use cases" href="/conductor/learn/overview/" />
  <LinkCard title="Quick Start" description="First workflow in 5 minutes" href="/conductor/quick-start/" />
  <LinkCard title="Installation" description="Homebrew, Go, or binary" href="/conductor/learn/installation/" />
  <LinkCard title="Examples" description="Ready-to-use workflows" href="/conductor/examples/" />
</CardGrid>

## Quick Example

\`\`\`yaml
name: summarize
steps:
  - id: summarize
    model: fast
    prompt: "Summarize: {{.inputs.text}}"
\`\`\`

\`\`\`bash
conductor run summarize.yaml -i text="Your long text here..."
\`\`\`

## Documentation

| Learn | Tutorials | Guides | Reference |
|-------|-----------|--------|-----------|
| [Installation](/conductor/learn/installation/) | [First Workflow](/conductor/learn/tutorials/first-workflow/) | [Flow Control](/conductor/guides/flow-control/) | [CLI](/conductor/reference/cli/) |
| [Workflows & Steps](/conductor/learn/concepts/workflows-steps/) | [Code Review Bot](/conductor/learn/tutorials/code-review-bot/) | [Error Handling](/conductor/guides/error-handling/) | [Workflow Schema](/conductor/reference/workflow-schema/) |
| [Inputs & Outputs](/conductor/learn/concepts/inputs-outputs/) | [Slack Integration](/conductor/learn/tutorials/slack-integration/) | [Performance](/conductor/guides/performance/) | [Configuration](/conductor/reference/configuration/) |
| [Template Variables](/conductor/learn/concepts/template-variables/) | [Multi-Agent](/conductor/learn/tutorials/multi-agent-workflows/) | [Testing](/conductor/guides/testing/) | [Error Codes](/conductor/reference/error-codes/) |

**Operations:** [File](/conductor/reference/connectors/file/) · [Shell](/conductor/reference/connectors/shell/) · [HTTP](/conductor/reference/connectors/http/) · [Transform](/conductor/reference/connectors/transform/)

**Service Integrations:** [GitHub](/conductor/reference/connectors/github/) · [Slack](/conductor/reference/connectors/slack/) · [Discord](/conductor/reference/connectors/discord/) · [Jira](/conductor/reference/connectors/jira/) · [Jenkins](/conductor/reference/connectors/jenkins/) · [Custom](/conductor/reference/connectors/custom/)

## Features

<CardGrid>
  <Card title="Workflow Logic in YAML" icon="pencil">
    Define what your agents do declaratively. The platform handles retries, fallbacks, and error handling.
  </Card>
  <Card title="Production-Ready" icon="approve-check-circle">
    Observability, cost tracking, and security built in.
  </Card>
  <Card title="Flexible Deployment" icon="rocket">
    Run from CLI, as an API, on a schedule, or via webhooks.
  </Card>
  <Card title="Any LLM Provider" icon="random">
    Anthropic, OpenAI, Ollama, or others. Swap without changing workflows.
  </Card>
</CardGrid>
`;
  writeFileSync(join(DOCS_DEST, 'index.mdx'), content);
  console.log('✓ index.mdx (homepage)');
}

/**
 * Discover examples with README.md and extract metadata
 */
function discoverExamples() {
  const examples = [];

  if (!existsSync(EXAMPLES_SRC)) {
    console.log('⚠ No examples/ directory found');
    return examples;
  }

  const entries = readdirSync(EXAMPLES_SRC);

  for (const entry of entries) {
    const examplePath = join(EXAMPLES_SRC, entry);
    const readmePath = join(examplePath, 'README.md');
    const workflowPath = join(examplePath, 'workflow.yaml');
    const stat = statSync(examplePath);

    if (!stat.isDirectory()) continue;
    if (entry === 'recipes' || entry === 'workflows') continue; // Skip these special dirs
    if (!existsSync(readmePath)) {
      console.log(`⚠ ${entry}/ missing README.md (skipping)`);
      continue;
    }

    const content = readFileSync(readmePath, 'utf-8');
    const title = extractTitle(content);
    const category = EXAMPLE_CATEGORIES[entry] || 'templates';

    // Extract first paragraph as description
    const descMatch = content.match(/^#.+\n+([^#\n][^\n]+)/m);
    const description = descMatch ? descMatch[1].trim() : '';

    // Check for workflow.yaml
    const hasWorkflow = existsSync(workflowPath);
    const workflowContent = hasWorkflow ? readFileSync(workflowPath, 'utf-8') : null;

    examples.push({
      name: entry,
      title,
      description,
      category,
      readmePath,
      workflowPath: hasWorkflow ? workflowPath : null,
      workflowContent,
    });
  }

  return examples;
}

/**
 * Process examples and generate index pages
 */
function processExamples() {
  const examples = discoverExamples();
  if (examples.length === 0) return 0;

  console.log('\nProcessing examples...');

  // Group by category
  const byCategory = {};
  for (const example of examples) {
    if (!byCategory[example.category]) {
      byCategory[example.category] = [];
    }
    byCategory[example.category].push(example);
  }

  let count = 0;

  // Process each example
  for (const example of examples) {
    const destDir = join(DOCS_DEST, 'examples', example.category);
    const destPath = join(destDir, `${example.name}.md`);

    // Process the README content and inject workflow.yaml
    processExampleFile(example.readmePath, destPath, example.workflowContent);
    console.log(`✓ examples/${example.category}/${example.name}.md`);
    count++;
  }

  // Generate category index pages
  const categoryMeta = {
    'code-review': { title: 'Code Review Examples', desc: 'AI-powered code review workflows' },
    'devops': { title: 'DevOps Examples', desc: 'Infrastructure and security workflows' },
    'automation': { title: 'Automation Examples', desc: 'Workflow automation and notifications' },
    'templates': { title: 'Templates', desc: 'Starter templates and simple examples' },
  };

  for (const [category, catExamples] of Object.entries(byCategory)) {
    const meta = categoryMeta[category] || { title: category, desc: '' };
    const destPath = join(DOCS_DEST, 'examples', category, 'index.md');

    let indexContent = `---\ntitle: "${meta.title}"\n---\n\n`;
    indexContent += `${meta.desc}\n\n`;
    indexContent += `| Example | Description |\n|---------|-------------|\n`;

    for (const ex of catExamples) {
      indexContent += `| [${ex.title}](${ex.name}/) | ${ex.description.slice(0, 80)}${ex.description.length > 80 ? '...' : ''} |\n`;
    }

    mkdirSync(dirname(destPath), { recursive: true });
    writeFileSync(destPath, indexContent);
    console.log(`✓ examples/${category}/index.md (auto-generated)`);
  }

  // Generate main examples index
  const mainIndexPath = join(DOCS_DEST, 'examples', 'index.md');
  let mainIndex = `---\ntitle: "Examples"\n---\n\n`;
  mainIndex += `Ready-to-use workflow examples organized by use case.\n\n`;

  for (const [category, catExamples] of Object.entries(byCategory)) {
    const meta = categoryMeta[category] || { title: category, desc: '' };
    mainIndex += `## ${meta.title}\n\n`;
    mainIndex += `| Example | Description |\n|---------|-------------|\n`;
    for (const ex of catExamples) {
      mainIndex += `| [${ex.title}](${category}/${ex.name}/) | ${ex.description.slice(0, 80)}${ex.description.length > 80 ? '...' : ''} |\n`;
    }
    mainIndex += `\n`;
  }

  mainIndex += `---\n\nSee [Recipes](../recipes/) for deployment patterns (webhooks, bots, APIs).\n`;
  writeFileSync(mainIndexPath, mainIndex);
  console.log(`✓ examples/index.md (auto-generated)`);

  return count;
}

/**
 * Main
 */
function main() {
  console.log('Processing docs/ for Starlight...\n');

  // Clean output directory to remove stale files
  if (existsSync(DOCS_DEST)) {
    rmSync(DOCS_DEST, { recursive: true });
  }
  mkdirSync(DOCS_DEST, { recursive: true });
  createHomepage();
  const docCount = processDirectory(DOCS_SRC, DOCS_DEST, true);
  const exampleCount = processExamples();

  console.log(`\n✓ Processed ${docCount} docs + ${exampleCount} examples`);
}

main();
