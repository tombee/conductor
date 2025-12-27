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

/*
Package rpc provides a WebSocket-based RPC server for real-time communication.

This package implements a bidirectional RPC protocol over WebSocket connections,
enabling clients to make requests to the daemon and receive streaming responses.
It's primarily used for interactive CLI sessions and agent communication.

# Overview

The RPC server supports:

  - Request/response messaging with correlation IDs
  - Streaming responses for long-running operations
  - Token-based authentication
  - Multiple concurrent connections

# Server Setup

Create and start an RPC server:

	cfg := &rpc.ServerConfig{
	    Port:      9876,
	    AuthToken: "secret-token",
	    Logger:    slog.Default(),
	}

	server := rpc.NewServer(cfg)
	if err := server.Start(); err != nil {
	    log.Fatal(err)
	}

# Handlers

Register handlers for different message types:

  - LLM Handlers: Process LLM completion requests
  - Tool Handlers: Execute tool calls
  - Workflow Handlers: Submit and manage workflow runs
  - Agent Handlers: Handle agent-specific operations
  - Cost Handlers: Track and report cost information

# Protocol

Messages follow a JSON-RPC-like format:

	// Request
	{
	    "id": "req-123",
	    "method": "llm.complete",
	    "params": {...}
	}

	// Response
	{
	    "id": "req-123",
	    "result": {...}
	}

	// Error
	{
	    "id": "req-123",
	    "error": {
	        "code": 400,
	        "message": "invalid request"
	    }
	}

# Authentication

When AuthToken is configured, clients must provide the token:

	// Via header during WebSocket upgrade
	Authorization: Bearer <token>

	// Or as query parameter
	?token=<token>

# Connection Lifecycle

 1. Client connects via WebSocket
 2. Server validates authentication (if enabled)
 3. Bidirectional message exchange
 4. Either side can close the connection
 5. Server tracks active connections for graceful shutdown

# Graceful Shutdown

The server supports graceful shutdown:

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
	    log.Printf("Shutdown error: %v", err)
	}

Active connections are closed with a close frame.
*/
package rpc
