# Conductor YAML Syntax Highlighting

This document describes the custom TextMate grammar used for Conductor workflow syntax highlighting in the documentation site.

## Overview

The Conductor documentation uses a custom TextMate grammar (`conductor.tmLanguage.json`) to provide syntax highlighting for workflow YAML files. This grammar:

- Highlights Conductor-specific keywords distinctly from YAML structure
- Recognizes connector shorthand syntax (`connector.operation:`)
- Highlights Go template variables and control flow
- Distinguishes enumerated values (step types, model tiers, error strategies)
- Auto-generates keyword patterns from the JSON Schema

## Usage in Documentation

Use the `conductor` language identifier in markdown code fences:

````markdown
```conductor
name: example-workflow
steps:
  - id: step1
    type: llm
    model: fast
    prompt: "Process {{.inputs.data}}"
```
````

## Grammar Features

### 1. Top-Level Keywords
Workflow root-level keys like `name`, `steps`, `inputs`, etc. are highlighted as primary keywords:

```conductor
name: my-workflow
description: Example workflow
inputs:
  - name: user_input
steps:
  - id: process
```

### 2. Step Configuration Keywords
Step-level properties like `id`, `type`, `model`, `prompt` are highlighted as secondary keywords:

```conductor
steps:
  - id: analyze
    type: llm
    model: strategic
    prompt: "Analyze this data"
```

### 3. Connector Shorthand
The `connector.operation:` pattern is recognized and highlighted:

```conductor
steps:
  - shell.run:
      command: ["echo", "hello"]

  - github.create_issue:
      repo: "owner/repo"
      title: "Bug report"
```

### 4. Template Syntax
Go template variables and control flow within strings:

```conductor
prompt: |
  User input: {{.inputs.name}}

  {{if .steps.analysis.response}}
  Previous analysis: {{.steps.analysis.response | truncate 500}}
  {{end}}

  {{range .inputs.items}}
  - Item: {{.}}
  {{end}}
```

### 5. Enumerated Values
Recognized constant values are highlighted distinctly:

```conductor
steps:
  - type: llm          # Step type enum
    model: fast        # Model tier enum
    on_error:
      strategy: retry  # Error strategy enum

inputs:
  - type: string       # Input type enum
```

## Grammar Maintenance

### Auto-Generation

The grammar keywords are automatically generated from `schemas/workflow.schema.json`:

```bash
npm run build:grammar
```

This script:
1. Parses the JSON Schema to extract property names
2. Categorizes them by nesting level (top-level, step-level, nested config)
3. Extracts enum values from schema definitions
4. Updates the grammar file's keyword patterns

### Build Integration

The grammar generation is integrated into the documentation build pipeline:

```bash
npm run build  # Runs build:grammar + convert + astro build
```

### Manual Updates

Some grammar patterns are hand-written and preserved during auto-generation:
- Connector shorthand regex pattern
- Template syntax begin/end patterns
- Template control flow and function highlighting
- Template variable path recognition

## TextMate Scopes

The grammar uses standard TextMate scope naming conventions:

| Construct | Scope | Theme Color |
|-----------|-------|-------------|
| Top-level keywords | `keyword.control.conductor` | Control keyword color |
| Step keywords | `keyword.other.conductor` | Secondary keyword color |
| Connector name | `entity.name.namespace.conductor` | Namespace color |
| Operation name | `entity.name.function.conductor` | Function color |
| Template delimiters | `punctuation.section.embedded` | Punctuation color |
| Template variables | `variable.other.conductor` | Variable color |
| Template functions | `support.function.conductor` | Function color |
| Enum values | `constant.language.conductor` | Constant color |
| Env var refs | `variable.other.environment` | Environment var color |

These scopes map to Starlight's built-in themes automatically - no custom CSS is required.

## VS Code Extension Support

The grammar file is designed to be reusable in VS Code extensions:

1. The grammar is a standard TextMate grammar in JSON format
2. All scope names follow TextMate conventions
3. The grammar includes YAML as a base language
4. Patterns use Oniguruma regex (compatible with VS Code)

To use in a VS Code extension:
1. Copy `docs-site/grammars/conductor.tmLanguage.json` to your extension
2. Register it in `package.json`:

```json
{
  "contributes": {
    "languages": [{
      "id": "conductor",
      "aliases": ["Conductor YAML", "conductor"],
      "extensions": [".conductor.yaml", ".conductor.yml"],
      "configuration": "./language-configuration.json"
    }],
    "grammars": [{
      "language": "conductor",
      "scopeName": "source.conductor",
      "path": "./grammars/conductor.tmLanguage.json"
    }]
  }
}
```

## Testing

### Build-Time Validation

The grammar is validated during the documentation build:
- Shiki loads and parses the grammar
- Build fails if the grammar has syntax errors
- All code blocks with `conductor` fence render successfully

### Manual Testing

1. Run the development server:
   ```bash
   npm run dev
   ```

2. Navigate to any documentation page with workflow examples

3. Verify highlighting in both light and dark themes:
   - Keywords should be distinct from values
   - Connector shorthand should be highlighted as a unit
   - Template variables should be visually distinct
   - Enum values should be highlighted consistently

### Performance Testing

Build time impact is monitored:
```bash
# Baseline (without grammar)
time npm run convert && astro build

# With grammar
time npm run build
```

Acceptable increase: < 5% of total build time

## Troubleshooting

### Grammar Not Loading

If the grammar doesn't load:
1. Check `docs-site/grammars/conductor.tmLanguage.json` exists
2. Verify `astro.config.mjs` imports and registers the grammar
3. Check browser console for Shiki errors
4. Ensure `build:grammar` was run before build

### Keywords Not Highlighting

If keywords aren't highlighted:
1. Run `npm run build:grammar` to regenerate patterns
2. Check the JSON Schema has the expected property names
3. Verify code blocks use ` ```conductor ` fence (not `yaml`)
4. Check grammar regex patterns in `conductor.tmLanguage.json`

### Template Syntax Issues

If templates aren't highlighting:
1. Ensure delimiters are `{{` and `}}` (not single braces)
2. Check template variables start with `.` (e.g., `.inputs.name`)
3. Verify the template is within a YAML string value
4. Check browser console for regex errors

## Future Enhancements

Potential improvements:
- Highlight known connector names differently from unknown ones
- Add semantic validation (e.g., warn on unknown template functions)
- Support for YAML anchors and aliases
- Multi-line template syntax support
- Schema-aware property value validation

## References

- [TextMate Grammar Spec](https://macromates.com/manual/en/language_grammars)
- [Shiki Documentation](https://shiki.style/)
- [astro-expressive-code](https://github.com/expressive-code/expressive-code)
- [Conductor Workflow Schema](../schemas/workflow.schema.json)
