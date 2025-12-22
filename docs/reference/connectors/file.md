# File

The `file` connector provides comprehensive filesystem operations for reading, writing, and managing files and directories.

## Overview

The file connector is a **builtin connector** - it requires no configuration and is always available. It provides secure, sandboxed file operations with automatic format detection and built-in safety features.

## Path Resolution

The file connector supports several path prefixes for secure, organized file access:

| Prefix | Description | Example |
|--------|-------------|---------|
| `./` | Relative to workflow file location | `./config.json` |
| `$out/` | Workflow output directory | `$out/result.txt` |
| `$temp/` | Temporary directory (cleaned up after run) | `$temp/work.json` |
| `/absolute` | Absolute paths (requires `AllowAbsolute` config) | `/etc/config` |

**Security Notes:**
- Symlinks are blocked by default (configurable)
- Absolute paths are blocked by default (configurable)
- Path traversal attempts (`../`) are validated and blocked if outside allowed directories
- Maximum file size limits apply (default: 100MB read, 100MB write)

## Operations

### Read Operations

#### file.read

Auto-detects format based on file extension and returns parsed content.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Path to file to read |

**Auto-Detection:**
- `.json` → Parsed as JSON (returns object/array)
- `.yaml`, `.yml` → Parsed as YAML (returns object/array)
- `.csv` → Parsed as CSV (returns array of rows)
- All others → Returns raw text content

**File Size Limits:**
- Files > 100MB: Error (configurable via `MaxFileSize`)
- Files > 10MB: Skip parsing, return as text (configurable via `MaxParseSize`)

**Example:**

```yaml
steps:
  # Auto-parses based on extension
  - file.read: ./config.json        # Returns parsed JSON
  - file.read: ./data.yaml          # Returns parsed YAML
  - file.read: ./readme.txt         # Returns text string
```

**Response:**

Depends on file format. JSON/YAML return parsed structures, text returns string.

```yaml
# For config.json containing {"port": 8080}
steps:
  - id: load_config
    file.read: ./config.json

  - id: use_port
    type: llm
    prompt: "Port is {{.steps.load_config.port}}"
```

**Errors:**

| Error Type | When |
|------------|------|
| `FileNotFound` | File does not exist |
| `FileTooLarge` | File exceeds MaxFileSize |
| `ParseError` | File format is invalid |
| `PermissionDenied` | Insufficient permissions |

---

#### file.read_text

Read file as raw text without any parsing.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Path to file to read |

**Example:**

```yaml
- file.read_text: ./template.html
```

**Response:**

```yaml
response: "<!DOCTYPE html>\n<html>..."
metadata:
  path: /absolute/path/to/template.html
  size: 1024
```

**Features:**
- UTF-8 BOM is automatically stripped
- No size limit for parsing (MaxFileSize still applies)
- Always returns string

---

#### file.read_json

Read and parse JSON file.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Path to JSON file |
| `extract` | string | No | JSONPath expression to extract subset |

**Example:**

```yaml
# Read entire JSON file
- file.read_json: ./data.json

# Extract specific field using JSONPath
- file.read_json:
    path: ./data.json
    extract: "$.users[0].name"
```

**Response:**

```yaml
# Full file
response: {"users": [{"name": "Alice", "age": 30}]}

# With extract: "$.users[0].name"
response: "Alice"
```

**Errors:**

| Error Type | When |
|------------|------|
| `ParseError` | Invalid JSON syntax |
| `Validation` | JSONPath extraction failed |

---

#### file.read_yaml

Read and parse YAML file.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Path to YAML file |

**Example:**

```yaml
- file.read_yaml: ./config.yaml
```

**Response:**

```yaml
response: {"key": "value", "nested": {"field": 123}}
metadata:
  path: /absolute/path/to/config.yaml
  size: 512
```

---

#### file.read_csv

Read and parse CSV file.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Path to CSV file |
| `delimiter` | string | No | Field delimiter (default: `,`) |

**Example:**

```yaml
# Read CSV with comma delimiter
- file.read_csv: ./data.csv

# Read TSV with tab delimiter
- file.read_csv:
    path: ./data.tsv
    delimiter: "\t"
```

**Response:**

```yaml
response:
  - ["Name", "Age", "City"]
  - ["Alice", "30", "NYC"]
  - ["Bob", "25", "LA"]
```

CSV is returned as array of arrays (rows).

---

#### file.read_lines

Read file as array of lines.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Path to file |

**Example:**

```yaml
- file.read_lines: ./log.txt
```

**Response:**

```yaml
response:
  - "Line 1"
  - "Line 2"
  - "Line 3"
```

**Features:**
- Splits on `\n` (Unix and Windows line endings supported)
- Trailing empty line is removed
- Useful for processing logs or configuration files line-by-line

---

### Write Operations

#### file.write

Write content to file with auto-formatting based on extension.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Path to output file |
| `content` | any | Yes | Content to write |

**Auto-Formatting:**
- `.json` → Pretty-printed JSON (2-space indent)
- `.yaml`, `.yml` → Formatted YAML
- All others → Content must be string, written as-is

**Example:**

```yaml
# Write object as JSON
- file.write:
    path: $out/result.json
    content:
      status: success
      count: 42

# Write text file
- file.write:
    path: $out/report.txt
    content: "Analysis complete"
```

**Features:**
- **Atomic writes**: Uses temp file + rename for safety
- **Auto-creates parent directories**: No need for `mkdir` first
- **Overwrites existing files**: No merge, complete replacement

**Response:**

```yaml
response: null
metadata:
  path: /absolute/path/to/result.json
  bytes: 256
```

**Errors:**

| Error Type | When |
|------------|------|
| `Validation` | Invalid content for format (e.g., non-string for .txt) |
| `FileTooLarge` | Content exceeds MaxFileSize |
| `PermissionDenied` | Insufficient permissions |

---

#### file.write_text

Write raw text to file.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Path to output file |
| `content` | string | Yes | Text content to write |

**Example:**

```yaml
- file.write_text:
    path: $out/output.txt
    content: "{{.steps.generate.response}}"
```

---

#### file.write_json

Write content as pretty-printed JSON.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Path to output file |
| `content` | any | Yes | Content to serialize as JSON |

**Example:**

```yaml
- file.write_json:
    path: $out/data.json
    content:
      timestamp: "2025-01-15T10:00:00Z"
      results: [1, 2, 3]
```

**Output:**
```json
{
  "timestamp": "2025-01-15T10:00:00Z",
  "results": [1, 2, 3]
}
```

---

#### file.write_yaml

Write content as YAML.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Path to output file |
| `content` | any | Yes | Content to serialize as YAML |

**Example:**

```yaml
- file.write_yaml:
    path: $out/config.yaml
    content:
      server:
        port: 8080
        host: localhost
```

---

#### file.append

Append text to existing file (or create if doesn't exist).

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Path to file |
| `content` | string | Yes | Text to append |

**Example:**

```yaml
- file.append:
    path: $out/log.txt
    content: "[{{.timestamp}}] Event occurred\n"
```

**Use Cases:**
- Building logs incrementally
- Appending to CSV files
- Accumulating results across steps

**Note:** Unlike `write`, `append` is **not atomic**. For structured data, prefer reading, modifying, and writing.

---

#### file.render

Render Go template to file.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `template` | string | Yes | Path to template file |
| `output` | string | Yes | Path to output file |
| `data` | object | Yes | Data to pass to template |

**Example:**

**Template (./report-template.md):**
```
# Code Review Report

**Repository:** {{.repo}}
**Issues Found:** {{.issueCount}}

{{range .issues}}
- {{.severity}}: {{.description}}
{{end}}
```

**Workflow:**
```yaml
- file.render:
    template: ./report-template.md
    output: $out/report.md
    data:
      repo: "myorg/myrepo"
      issueCount: 2
      issues:
        - severity: "high"
          description: "SQL injection vulnerability"
        - severity: "medium"
          description: "Missing error handling"
```

**Security:**
- Template functions are **restricted** for safety
- Available functions: standard Go template functions (`range`, `if`, `with`, etc.)
- No access to system calls or arbitrary code execution

---

### Directory Operations

#### file.list

List files and directories with optional filtering.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Directory to list |
| `pattern` | string | No | Glob pattern to filter results |
| `recursive` | boolean | No | Recursively list subdirectories (default: `false`) |
| `type` | string | No | Filter by type: `all`, `files`, `dirs` (default: `all`) |

**Example:**

```yaml
# List all files in directory
- file.list:
    path: ./src

# List only .go files recursively
- file.list:
    path: ./src
    pattern: "*.go"
    recursive: true

# List only directories
- file.list:
    path: ./
    type: dirs
```

**Response:**

```yaml
response:
  - path: /absolute/path/to/file1.go
    name: file1.go
    size: 2048
    mode: "-rw-r--r--"
    modTime: "2025-01-15T10:30:00Z"
    isDir: false
  - path: /absolute/path/to/file2.go
    name: file2.go
    size: 1024
    mode: "-rw-r--r--"
    modTime: "2025-01-15T11:00:00Z"
    isDir: false
metadata:
  path: /absolute/path/to/src
  pattern: "*.go"
  count: 2
```

**Glob Patterns:**
- `*` - Matches any characters except `/`
- `**` - Matches any characters including `/` (requires `recursive: true`)
- `?` - Matches single character
- `[abc]` - Matches one of the characters

---

#### file.exists

Check if file or directory exists.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Path to check |

**Example:**

```yaml
- id: check_config
  file.exists:
    path: ./config.json

- id: warn_if_missing
  type: condition
  condition:
    expression: "$.check_config == false"
    then_steps: [create_default_config]
```

**Response:**

```yaml
response: true
metadata:
  path: /absolute/path/to/config.json
```

---

#### file.stat

Get file or directory information.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Path to check |

**Example:**

```yaml
- file.stat: ./large-file.dat
```

**Response:**

```yaml
response:
  path: /absolute/path/to/large-file.dat
  name: large-file.dat
  size: 1048576
  mode: "-rw-r--r--"
  modTime: "2025-01-15T10:00:00Z"
  isDir: false
metadata:
  path: /absolute/path/to/large-file.dat
```

---

#### file.mkdir

Create directory (with optional parent directories).

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Directory path to create |
| `parents` | boolean | No | Create parent directories if missing (default: `true`) |

**Example:**

```yaml
# Create directory and parents
- file.mkdir:
    path: $out/results/2025-01-15

# Create only if parent exists
- file.mkdir:
    path: ./subdir
    parents: false
```

**Response:**

```yaml
response: null
metadata:
  path: /absolute/path/to/results/2025-01-15
  created: true
```

**Idempotent:** If directory already exists, succeeds without error (`created: false`).

---

#### file.copy

Copy file or directory.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `src` | string | Yes | Source path |
| `dest` | string | Yes | Destination path |
| `recursive` | boolean | No | Copy directories recursively (default: `false`) |

**Example:**

```yaml
# Copy single file
- file.copy:
    src: ./template.yaml
    dest: $out/workflow.yaml

# Copy entire directory
- file.copy:
    src: ./configs
    dest: $out/configs
    recursive: true
```

**Response:**

```yaml
response: null
metadata:
  src: /absolute/path/to/template.yaml
  dest: /absolute/path/out/workflow.yaml
  bytes: 2048
```

**Behavior:**
- **Files**: Destination is the file itself (not a directory to copy into)
- **Directories**: Must use `recursive: true`
- **Overwrites**: Destination is replaced if it exists
- **Preserves permissions**: File mode bits are copied

---

#### file.move

Move or rename file or directory.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `src` | string | Yes | Source path |
| `dest` | string | Yes | Destination path |

**Example:**

```yaml
# Rename file
- file.move:
    src: $temp/draft.txt
    dest: $out/final.txt

# Move directory
- file.move:
    src: ./old-location
    dest: ./new-location
```

**Response:**

```yaml
response: null
metadata:
  src: /absolute/path/to/draft.txt
  dest: /absolute/path/out/final.txt
  method: "rename"
```

**Cross-Device Moves:**

If source and destination are on different filesystems, the connector automatically:
1. Copies all files to destination
2. Deletes source
3. Returns `method: "copy-delete"` in metadata

---

#### file.delete

Delete file or directory.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Path to delete |
| `recursive` | boolean | No | Delete directories recursively (default: `false`) |

**Example:**

```yaml
# Delete single file
- file.delete:
    path: $temp/work.json

# Delete directory and contents
- file.delete:
    path: $temp/cache
    recursive: true
```

**Response:**

```yaml
response: null
metadata:
  path: /absolute/path/temp/work.json
  deleted: true
```

**Safety:**
- Directories require `recursive: true` to delete
- If path doesn't exist, succeeds without error (`deleted: false`)
- **No undo** - deleted files cannot be recovered

---

## Complete Example

```yaml
name: file-processing-workflow
description: "Demonstrates file connector operations"

steps:
  # Read input data
  - id: load_config
    file.read_json:
      path: ./config.json

  # Check if output directory exists
  - id: check_output_dir
    file.exists:
      path: $out/reports

  # Create directory if missing
  - id: create_output_dir
    type: condition
    condition:
      expression: "$.check_output_dir == false"
      then_steps: [make_output_dir]

  - id: make_output_dir
    file.mkdir:
      path: $out/reports

  # Process files from input directory
  - id: list_input_files
    file.list:
      path: ./input
      pattern: "*.txt"
      type: files

  # Read and analyze first file
  - id: read_first_file
    file.read_text:
      path: "{{index .steps.list_input_files 0 \"path\"}}"

  - id: analyze
    type: llm
    prompt: "Analyze this text: {{.steps.read_first_file}}"

  # Write results
  - id: write_result
    file.write_json:
      path: $out/reports/analysis.json
      content:
        timestamp: "{{.timestamp}}"
        analysis: "{{.steps.analyze.response}}"
        fileCount: "{{len .steps.list_input_files}}"

  # Render report from template
  - id: render_report
    file.render:
      template: ./templates/report.md
      output: $out/reports/report.md
      data:
        analysis: "{{.steps.analyze.response}}"

  # Copy original files to archive
  - id: archive_inputs
    file.copy:
      src: ./input
      dest: $out/archive
      recursive: true

outputs:
  - name: result_path
    type: string
    value: "$.write_result.path"
```

---

## Error Handling

All file operations return consistent error types:

| Error Type | HTTP Equivalent | Retry? |
|------------|----------------|--------|
| `FileNotFound` | 404 | No |
| `FileTooLarge` | 413 | No |
| `PermissionDenied` | 403 | No |
| `ParseError` | 400 | No |
| `Validation` | 400 | No |
| `Internal` | 500 | Yes (transient failures) |

**Error Handling Example:**

```yaml
- id: try_read
  file.read: ./optional-config.json
  on_error:
    strategy: fallback
    fallback_step: use_defaults

- id: use_defaults
  file.write_json:
    path: ./config.json
    content:
      default: true
```

---

## Configuration

File connector behavior can be customized via workflow execution config (not in workflow YAML):

```yaml
# config.yaml (daemon/runtime config)
builtin_connectors:
  file:
    max_file_size: 104857600      # 100MB (default)
    max_parse_size: 10485760      # 10MB (default)
    allow_symlinks: false          # Default: false
    allow_absolute: false          # Default: false
```

**Security Considerations:**

- **Production**: Keep `allow_absolute: false` to prevent workflows from accessing arbitrary paths
- **Development**: Enable `allow_absolute: true` for flexibility, but review workflows carefully
- **Symlinks**: Enabling `allow_symlinks: true` can bypass path restrictions - use with caution

---

## Best Practices

### 1. Use Path Prefixes

```yaml
# GOOD - Organized, secure
- file.write: $out/result.json        # Clear it goes to output
- file.read: ./input.yaml             # Relative to workflow

# AVOID - Unclear, potentially insecure
- file.write: /tmp/result.json        # Absolute path (blocked by default)
```

### 2. Handle Missing Files

```yaml
# GOOD - Check before reading
- id: check_cache
  file.exists: $temp/cache.json

- id: read_cache
  type: condition
  condition:
    expression: "$.check_cache == true"
    then_steps: [load_cache]
    else_steps: [build_cache]

# AVOID - Fail on missing file
- file.read: $temp/cache.json   # Errors if missing
```

### 3. Prefer Explicit Format Operations

```yaml
# GOOD - Explicit about format
- file.read_json: ./config.json

# OKAY - Auto-detection works but less explicit
- file.read: ./config.json
```

### 4. Use Atomic Operations

```yaml
# GOOD - Atomic write (safe)
- file.write:
    path: $out/critical-data.json
    content: {{.results}}

# RISKY - Non-atomic append (can corrupt on failure mid-write)
- file.append:
    path: $out/critical-data.json
    content: "{{.results}}"
```

---

## Related

- [Workflow Schema Reference](../workflow-schema.md) - Complete workflow YAML schema
- [Shell Connector](shell.md) - Execute shell commands
- [HTTP Connector](http.md) - Make HTTP requests
- [Connectors Overview](../../learn/concepts/connectors-overview.md) - Working with connectors
