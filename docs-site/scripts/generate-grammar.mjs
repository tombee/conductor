#!/usr/bin/env node

/**
 * Generate Conductor TextMate Grammar Keywords
 *
 * Extracts keywords and enums from schemas/workflow.schema.json and updates
 * the conductor.tmLanguage.json grammar file to keep syntax highlighting
 * in sync with the schema.
 */

import { readFileSync, writeFileSync } from 'fs';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const projectRoot = join(__dirname, '../..');

// Paths
const schemaPath = join(projectRoot, 'schemas/workflow.schema.json');
const grammarPath = join(projectRoot, 'docs-site/grammars/conductor.tmLanguage.json');

// Load schema
const schema = JSON.parse(readFileSync(schemaPath, 'utf-8'));

/**
 * Extract property names from schema definitions
 */
function extractKeywords(schemaObj, path = []) {
  const keywords = {
    topLevel: new Set(),
    stepLevel: new Set(),
    nested: new Set(),
    enums: {
      stepTypes: new Set(),
      modelTiers: new Set(),
      errorStrategies: new Set(),
      inputTypes: new Set(),
    }
  };

  // Top-level workflow properties
  if (schemaObj.properties) {
    Object.keys(schemaObj.properties).forEach(key => {
      if (path.length === 0) {
        keywords.topLevel.add(key);
      }
    });
  }

  // Extract step-level keywords from step definition
  if (schemaObj.$defs?.step?.properties) {
    Object.keys(schemaObj.$defs.step.properties).forEach(key => {
      keywords.stepLevel.add(key);
    });
  }

  // Extract nested config keywords
  const nestedDefs = [
    'listen_config',
    'webhook_trigger',
    'schedule_trigger',
    'api_trigger',
    'agent',
    'mcp_server',
    'retry_config',
    'condition_config'
  ];

  nestedDefs.forEach(defName => {
    if (schemaObj.$defs?.[defName]?.properties) {
      Object.keys(schemaObj.$defs[defName].properties).forEach(key => {
        keywords.nested.add(key);
      });
    }
  });

  // Extract enum values
  // Step types
  if (schemaObj.$defs?.step?.properties?.type?.enum) {
    schemaObj.$defs.step.properties.type.enum.forEach(val => {
      keywords.enums.stepTypes.add(val);
    });
  }

  // Model tiers
  if (schemaObj.$defs?.step?.properties?.model?.enum) {
    schemaObj.$defs.step.properties.model.enum.forEach(val => {
      keywords.enums.modelTiers.add(val);
    });
  }

  // Error strategies
  if (schemaObj.$defs?.error_handling?.properties?.strategy?.enum) {
    schemaObj.$defs.error_handling.properties.strategy.enum.forEach(val => {
      keywords.enums.errorStrategies.add(val);
    });
  }

  // Input types
  if (schemaObj.$defs?.input?.properties?.type?.enum) {
    schemaObj.$defs.input.properties.type.enum.forEach(val => {
      keywords.enums.inputTypes.add(val);
    });
  }

  return keywords;
}

/**
 * Generate regex pattern from keyword set
 */
function generateKeywordPattern(keywords) {
  return Array.from(keywords).sort().join('|');
}

/**
 * Update grammar file with extracted keywords
 */
function updateGrammar(keywords) {
  const grammar = JSON.parse(readFileSync(grammarPath, 'utf-8'));

  // Update top-level keywords pattern
  const topLevelPattern = generateKeywordPattern(keywords.topLevel);
  const topLevelRule = grammar.repository['top-level-keywords'];
  if (topLevelRule) {
    topLevelRule.match = `^\\\\s*(${topLevelPattern})\\\\s*:`;
  }

  // Update step keywords pattern
  const stepPattern = generateKeywordPattern(keywords.stepLevel);
  const stepRule = grammar.repository['step-keywords'];
  if (stepRule) {
    stepRule.match = `^\\\\s+(${stepPattern})\\\\s*:`;
  }

  // Update nested config keywords
  const nestedPattern = generateKeywordPattern(keywords.nested);
  const nestedRules = grammar.repository['nested-config-keywords']?.patterns;
  if (nestedRules && nestedRules.length > 0) {
    // Update the first pattern to include all nested keywords
    nestedRules[0].match = `^\\\\s+(${nestedPattern})\\\\s*:`;
  }

  // Update enum patterns
  const enumPatterns = grammar.repository['enum-values']?.patterns;
  if (enumPatterns) {
    // Step types
    if (keywords.enums.stepTypes.size > 0) {
      const stepTypesPattern = generateKeywordPattern(keywords.enums.stepTypes);
      enumPatterns[0].match = `:\\\\s*(${stepTypesPattern})\\\\b`;
    }

    // Model tiers
    if (keywords.enums.modelTiers.size > 0) {
      const modelTiersPattern = generateKeywordPattern(keywords.enums.modelTiers);
      enumPatterns[1].match = `:\\\\s*(${modelTiersPattern})\\\\b`;
    }

    // Error strategies
    if (keywords.enums.errorStrategies.size > 0) {
      const errorStrategiesPattern = generateKeywordPattern(keywords.enums.errorStrategies);
      enumPatterns[2].match = `:\\\\s*(${errorStrategiesPattern})\\\\b`;
    }

    // Input types
    if (keywords.enums.inputTypes.size > 0) {
      const inputTypesPattern = generateKeywordPattern(keywords.enums.inputTypes);
      enumPatterns[3].match = `:\\\\s*(${inputTypesPattern})\\\\b`;
    }
  }

  // Write updated grammar
  writeFileSync(grammarPath, JSON.stringify(grammar, null, 2) + '\n', 'utf-8');
}

/**
 * Main execution
 */
function main() {
  console.log('Generating Conductor grammar keywords from schema...');
  console.log(`Schema: ${schemaPath}`);
  console.log(`Grammar: ${grammarPath}`);

  try {
    const keywords = extractKeywords(schema);

    console.log(`\nExtracted keywords:`);
    console.log(`  Top-level: ${keywords.topLevel.size} keywords`);
    console.log(`  Step-level: ${keywords.stepLevel.size} keywords`);
    console.log(`  Nested config: ${keywords.nested.size} keywords`);
    console.log(`  Enum values: ${keywords.enums.stepTypes.size + keywords.enums.modelTiers.size + keywords.enums.errorStrategies.size + keywords.enums.inputTypes.size} total`);

    updateGrammar(keywords);

    console.log('\nGrammar updated successfully!');
  } catch (error) {
    console.error('Error generating grammar:', error);
    process.exit(1);
  }
}

main();
