// Copyright 2025 Tom Barlow
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mcp

// getPythonTemplateFiles returns the template files for Python MCP servers.
func getPythonTemplateFiles(template string) map[string]string {
	files := map[string]string{
		"requirements.txt": `mcp>=0.1.0
`,
		"README.md": `# {{.Name}} MCP Server

A Model Context Protocol (MCP) server built with Python.

## Installation

` + "```bash" + `
pip install -r requirements.txt
` + "```" + `

## Usage

Run the server:

` + "```bash" + `
python server.py
` + "```" + `

Use in a Conductor workflow:

` + "```yaml" + `
mcp_servers:
  - name: {{.Name}}
    command: python
    args: ["/path/to/{{.Name}}/server.py"]

steps:
  - id: example
    type: llm
    prompt: "test"
    inputs:
      param: "value"
` + "```" + `

## Available Tools

See server.py for the list of available tools and their parameters.
`,
	}

	// Add template-specific server.py
	switch template {
	case "blank":
		files["server.py"] = getPythonBlankTemplate()
	case "http":
		files["server.py"] = getPythonHTTPTemplate()
	case "database":
		files["server.py"] = getPythonDatabaseTemplate()
	}

	return files
}

// getTypeScriptTemplateFiles returns the template files for TypeScript MCP servers.
func getTypeScriptTemplateFiles(template string) map[string]string {
	files := map[string]string{
		"package.json": `{
  "name": "{{.Identifier}}",
  "version": "1.0.0",
  "description": "{{.Name}} MCP Server",
  "main": "dist/server.js",
  "scripts": {
    "build": "tsc",
    "dev": "tsx server.ts",
    "start": "node dist/server.js"
  },
  "dependencies": {
    "@modelcontextprotocol/sdk": "^0.1.0"
  },
  "devDependencies": {
    "@types/node": "^20.0.0",
    "tsx": "^4.0.0",
    "typescript": "^5.0.0"
  }
}
`,
		"tsconfig.json": `{
  "compilerOptions": {
    "target": "ES2020",
    "module": "commonjs",
    "outDir": "./dist",
    "rootDir": "./",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true
  },
  "include": ["*.ts"],
  "exclude": ["node_modules", "dist"]
}
`,
		"README.md": `# {{.Name}} MCP Server

A Model Context Protocol (MCP) server built with TypeScript.

## Installation

` + "```bash" + `
npm install
` + "```" + `

## Usage

Development mode:

` + "```bash" + `
npm run dev
` + "```" + `

Build and run:

` + "```bash" + `
npm run build
npm start
` + "```" + `

Use in a Conductor workflow:

` + "```yaml" + `
mcp_servers:
  - name: {{.Name}}
    command: node
    args: ["/path/to/{{.Name}}/dist/server.js"]

steps:
  - id: example
    type: llm
    prompt: "test"
    inputs:
      param: "value"
` + "```" + `

## Available Tools

See server.ts for the list of available tools and their parameters.
`,
		"server.ts": getTypeScriptBlankTemplate(),
	}

	return files
}
func getPythonBlankTemplate() string {
	return `#!/usr/bin/env python3
"""
{{.Name}} MCP Server

A basic MCP server implementation.
"""

import json
import sys
from typing import Any


def handle_initialize(request_id: int) -> dict[str, Any]:
    """Handle the initialize request."""
    return {
        "jsonrpc": "2.0",
        "id": request_id,
        "result": {
            "protocolVersion": "0.1.0",
            "capabilities": {
                "tools": {}
            },
            "serverInfo": {
                "name": "{{.Name}}",
                "version": "1.0.0"
            }
        }
    }


def handle_tools_list(request_id: int) -> dict[str, Any]:
    """Handle the tools/list request."""
    return {
        "jsonrpc": "2.0",
        "id": request_id,
        "result": {
            "tools": [
                {
                    "name": "example_tool",
                    "description": "An example tool that echoes input",
                    "inputSchema": {
                        "type": "object",
                        "properties": {
                            "message": {
                                "type": "string",
                                "description": "The message to echo"
                            }
                        },
                        "required": ["message"]
                    }
                }
            ]
        }
    }


def handle_tools_call(request_id: int, params: dict[str, Any]) -> dict[str, Any]:
    """Handle the tools/call request."""
    tool_name = params.get("name")
    arguments = params.get("arguments", {})

    if tool_name == "example_tool":
        message = arguments.get("message", "")
        return {
            "jsonrpc": "2.0",
            "id": request_id,
            "result": {
                "content": [
                    {
                        "type": "text",
                        "text": f"Echo: {message}"
                    }
                ]
            }
        }

    return {
        "jsonrpc": "2.0",
        "id": request_id,
        "error": {
            "code": -32601,
            "message": f"Unknown tool: {tool_name}"
        }
    }


def main():
    """Main server loop."""
    for line in sys.stdin:
        try:
            request = json.loads(line.strip())
            method = request.get("method")
            request_id = request.get("id")
            params = request.get("params", {})

            if method == "initialize":
                response = handle_initialize(request_id)
            elif method == "tools/list":
                response = handle_tools_list(request_id)
            elif method == "tools/call":
                response = handle_tools_call(request_id, params)
            else:
                response = {
                    "jsonrpc": "2.0",
                    "id": request_id,
                    "error": {
                        "code": -32601,
                        "message": f"Method not found: {method}"
                    }
                }

            print(json.dumps(response), flush=True)

        except json.JSONDecodeError as e:
            print(json.dumps({
                "jsonrpc": "2.0",
                "id": None,
                "error": {
                    "code": -32700,
                    "message": f"Parse error: {e}"
                }
            }), flush=True)
        except Exception as e:
            print(json.dumps({
                "jsonrpc": "2.0",
                "id": None,
                "error": {
                    "code": -32603,
                    "message": f"Internal error: {e}"
                }
            }), flush=True)


if __name__ == "__main__":
    main()
`
}

func getPythonHTTPTemplate() string {
	return `#!/usr/bin/env python3
"""
{{.Name}} MCP Server

An HTTP API wrapper MCP server.
"""

import json
import sys
from typing import Any
import urllib.request
import urllib.error


def handle_initialize(request_id: int) -> dict[str, Any]:
    """Handle the initialize request."""
    return {
        "jsonrpc": "2.0",
        "id": request_id,
        "result": {
            "protocolVersion": "0.1.0",
            "capabilities": {
                "tools": {}
            },
            "serverInfo": {
                "name": "{{.Name}}",
                "version": "1.0.0"
            }
        }
    }


def handle_tools_list(request_id: int) -> dict[str, Any]:
    """Handle the tools/list request."""
    return {
        "jsonrpc": "2.0",
        "id": request_id,
        "result": {
            "tools": [
                {
                    "name": "http_get",
                    "description": "Make an HTTP GET request",
                    "inputSchema": {
                        "type": "object",
                        "properties": {
                            "url": {
                                "type": "string",
                                "description": "The URL to request"
                            }
                        },
                        "required": ["url"]
                    }
                }
            ]
        }
    }


def handle_tools_call(request_id: int, params: dict[str, Any]) -> dict[str, Any]:
    """Handle the tools/call request."""
    tool_name = params.get("name")
    arguments = params.get("arguments", {})

    if tool_name == "http_get":
        url = arguments.get("url", "")
        try:
            with urllib.request.urlopen(url) as response:
                data = response.read().decode('utf-8')
                return {
                    "jsonrpc": "2.0",
                    "id": request_id,
                    "result": {
                        "content": [
                            {
                                "type": "text",
                                "text": data
                            }
                        ]
                    }
                }
        except urllib.error.URLError as e:
            return {
                "jsonrpc": "2.0",
                "id": request_id,
                "error": {
                    "code": -32000,
                    "message": f"HTTP request failed: {e}"
                }
            }

    return {
        "jsonrpc": "2.0",
        "id": request_id,
        "error": {
            "code": -32601,
            "message": f"Unknown tool: {tool_name}"
        }
    }


def main():
    """Main server loop."""
    for line in sys.stdin:
        try:
            request = json.loads(line.strip())
            method = request.get("method")
            request_id = request.get("id")
            params = request.get("params", {})

            if method == "initialize":
                response = handle_initialize(request_id)
            elif method == "tools/list":
                response = handle_tools_list(request_id)
            elif method == "tools/call":
                response = handle_tools_call(request_id, params)
            else:
                response = {
                    "jsonrpc": "2.0",
                    "id": request_id,
                    "error": {
                        "code": -32601,
                        "message": f"Method not found: {method}"
                    }
                }

            print(json.dumps(response), flush=True)

        except json.JSONDecodeError as e:
            print(json.dumps({
                "jsonrpc": "2.0",
                "id": None,
                "error": {
                    "code": -32700,
                    "message": f"Parse error: {e}"
                }
            }), flush=True)
        except Exception as e:
            print(json.dumps({
                "jsonrpc": "2.0",
                "id": None,
                "error": {
                    "code": -32603,
                    "message": f"Internal error: {e}"
                }
            }), flush=True)


if __name__ == "__main__":
    main()
`
}

func getPythonDatabaseTemplate() string {
	return `#!/usr/bin/env python3
"""
{{.Name}} MCP Server

A database query MCP server.
"""

import json
import sys
from typing import Any


def handle_initialize(request_id: int) -> dict[str, Any]:
    """Handle the initialize request."""
    return {
        "jsonrpc": "2.0",
        "id": request_id,
        "result": {
            "protocolVersion": "0.1.0",
            "capabilities": {
                "tools": {}
            },
            "serverInfo": {
                "name": "{{.Name}}",
                "version": "1.0.0"
            }
        }
    }


def handle_tools_list(request_id: int) -> dict[str, Any]:
    """Handle the tools/list request."""
    return {
        "jsonrpc": "2.0",
        "id": request_id,
        "result": {
            "tools": [
                {
                    "name": "query",
                    "description": "Execute a database query",
                    "inputSchema": {
                        "type": "object",
                        "properties": {
                            "sql": {
                                "type": "string",
                                "description": "The SQL query to execute"
                            }
                        },
                        "required": ["sql"]
                    }
                }
            ]
        }
    }


def handle_tools_call(request_id: int, params: dict[str, Any]) -> dict[str, Any]:
    """Handle the tools/call request."""
    tool_name = params.get("name")
    arguments = params.get("arguments", {})

    if tool_name == "query":
        sql = arguments.get("sql", "")
        # TODO: Implement actual database connection
        # This is a placeholder that returns example data
        return {
            "jsonrpc": "2.0",
            "id": request_id,
            "result": {
                "content": [
                    {
                        "type": "text",
                        "text": f"Query executed: {sql}\nResults: [example data]"
                    }
                ]
            }
        }

    return {
        "jsonrpc": "2.0",
        "id": request_id,
        "error": {
            "code": -32601,
            "message": f"Unknown tool: {tool_name}"
        }
    }


def main():
    """Main server loop."""
    for line in sys.stdin:
        try:
            request = json.loads(line.strip())
            method = request.get("method")
            request_id = request.get("id")
            params = request.get("params", {})

            if method == "initialize":
                response = handle_initialize(request_id)
            elif method == "tools/list":
                response = handle_tools_list(request_id)
            elif method == "tools/call":
                response = handle_tools_call(request_id, params)
            else:
                response = {
                    "jsonrpc": "2.0",
                    "id": request_id,
                    "error": {
                        "code": -32601,
                        "message": f"Method not found: {method}"
                    }
                }

            print(json.dumps(response), flush=True)

        except json.JSONDecodeError as e:
            print(json.dumps({
                "jsonrpc": "2.0",
                "id": None,
                "error": {
                    "code": -32700,
                    "message": f"Parse error: {e}"
                }
            }), flush=True)
        except Exception as e:
            print(json.dumps({
                "jsonrpc": "2.0",
                "id": None,
                "error": {
                    "code": -32603,
                    "message": f"Internal error: {e}"
                }
            }), flush=True)


if __name__ == "__main__":
    main()
`
}

func getTypeScriptBlankTemplate() string {
	return `import { Server } from '@modelcontextprotocol/sdk/server/index.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
} from '@modelcontextprotocol/sdk/types.js';

// Create server instance
const server = new Server(
  {
    name: '{{.Name}}',
    version: '1.0.0',
  },
  {
    capabilities: {
      tools: {},
    },
  }
);

// Register tool handlers
server.setRequestHandler(ListToolsRequestSchema, async () => {
  return {
    tools: [
      {
        name: 'example_tool',
        description: 'An example tool that echoes input',
        inputSchema: {
          type: 'object',
          properties: {
            message: {
              type: 'string',
              description: 'The message to echo',
            },
          },
          required: ['message'],
        },
      },
    ],
  };
});

server.setRequestHandler(CallToolRequestSchema, async (request) => {
  const { name, arguments: args } = request.params;

  if (name === 'example_tool') {
    const message = (args as any).message || '';
    return {
      content: [
        {
          type: 'text',
          text: ` + "`Echo: ${message}`" + `,
        },
      ],
    };
  }

  throw new Error(` + "`Unknown tool: ${name}`" + `);
});

// Start server
async function main() {
  const transport = new StdioServerTransport();
  await server.connect(transport);
  console.error('{{.NameTitle}} MCP server running on stdio');
}

main().catch((error) => {
  console.error('Server error:', error);
  process.exit(1);
});
`
}
