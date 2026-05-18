# MCP Server Integration Specification

## Overview

The Model Context Protocol (MCP) is an open protocol that standardizes how applications provide context to LLM providers. This specification defines how the coding agent integrates with MCP servers to extend its capabilities through external tools, resources, and prompts.

MCP enables the coding agent to:

- **Tools**: Execute functions exposed by external MCP servers (e.g., database queries, API calls, specialized tools)
- **Resources**: Access contextual data from external sources (e.g., files, database records, APIs)
- **Prompts**: Use pre-defined prompt templates from MCP servers

## Architecture

```
┌─────────────────────────────────────────────────┐
│                  Coding Agent                   │
│  ┌─────────────┐    ┌────────────────────────┐  │
│  │   Agent     │    │   MCP Client Manager   │  │
│  │   Core      │◄──►│                        │  │
│  │             │    │  ┌──────────────────┐  │  │
│  │  Inference  │    │  │ MCP Connection 1 │  │  │
│  │  Backend    │    │  └──────────────────┘  │  │
│  │             │    │  ┌──────────────────┐  │  │
│  │  Built-in   │    │  │ MCP Connection 2 │  │  │
│  │  Tools      │    │  └──────────────────┘  │  │
│  └─────────────┘    └────────────────────────┘  │
└──────────────┬──────────────────────────────────┘
               │ JSON-RPC 2.0
               │ (stdio / SSE)
    ┌──────────┴──────────┐
    │                     │
┌───▼─────┐       ┌──────▼──────┐
│ MCP     │       │ MCP Server 2│
│ Server 1│       │ (e.g., DB)  │
│ (e.g.,  │       │             │
│ FS)     │       └─────────────┘
└─────────┘
```

The MCP Client Manager handles:

- Lifecycle management of MCP server connections
- Capability negotiation during initialization
- Routing tool/resource/prompt requests to appropriate servers
- Error handling and reconnection logic

## Transport Layer

### Transport Types

MCP supports multiple transport mechanisms:

| Transport                    | Description                | Use Case                               |
| ---------------------------- | -------------------------- | -------------------------------------- |
| **stdio**                    | Standard I/O communication | Local MCP servers, process-based tools |
| **SSE** (Server-Sent Events) | HTTP-based streaming       | Remote MCP servers, network services   |

### stdio Transport

For stdio transport, the coding agent spawns a subprocess and communicates via JSON-RPC over stdin/stdout.

```
Agent Process              MCP Server Process
     │                           │
     │──── stdio (write) ───────►│  InitializeRequest
     │                           │
     │◄──── stdio (read) ────────│  InitializeResult
     │                           │
     │──── stdio (write) ───────►│  Tools/Call requests
     │                           │
     │◄──── stdio (read) ────────│  Tools/Call responses
     │                           │
```

#### Configuration

```ini
# MCP Server via stdio
[mcp_server.file_system]
transport=stdio
command=npx
args=-y @modelcontextprotocol/server-filesystem /home/user/projects
timeout=30
```

#### stdio Message Format

Each message is a JSON-RPC 2.0 object terminated by a newline:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2024-11-05",
    "capabilities": {},
    "clientInfo": { "name": "coding-agent", "version": "0.1.0" }
  }
}
```

### SSE Transport

For SSE transport, the coding agent connects to an HTTP endpoint and communicates via HTTP POST (requests) and SSE (responses).

```
Agent Process              MCP Server (HTTP)
     │                            │
     │──── HTTP POST ────────────►│  /messages (InitializeRequest)
     │                            │
     │◄──── SSE Event ────────────│  InitializeResult (on /sse endpoint)
     │                            │
     │──── HTTP POST ────────────►│  /messages (Tools/Call request)
     │                            │
     │◄──── SSE Event ────────────│  Tools/Call response
     │                            │
```

#### Configuration

```ini
# MCP Server via SSE
[mcp_server.database]
transport=sse
url=http://localhost:3001/sse
endpoint=http://localhost:3001/messages
timeout=30
```

## Protocol Primitives

MCP uses JSON-RPC 2.0 as its transport protocol. All messages follow the JSON-RPC 2.0 specification.

### JSON-RPC Message Types

#### Request

```typescript
interface McpRequest {
  jsonrpc: "2.0";
  id: number | string; // Unique request identifier
  method: string; // Method name
  params?: object; // Method parameters
}
```

#### Notification (No Response Expected)

```typescript
interface McpNotification {
  jsonrpc: "2.0";
  method: string;
  params?: object;
  // Note: No 'id' field for notifications
}
```

#### Response (Success)

```typescript
interface McpResponse {
  jsonrpc: "2.0";
  id: number | string; // Matches request id
  result: object;
}
```

#### Response (Error)

```typescript
interface McpErrorResponse {
  jsonrpc: "2.0";
  id: number | string; // Matches request id
  error: {
    code: number; // Error code
    message: string; // Error message
    data?: object; // Optional error details
  };
}
```

## Lifecycle

The MCP lifecycle consists of four phases:

```
┌───────────┐    ┌──────────────┐    ┌───────────┐    ┌──────────┐
│  Connect  │───►│   Initialize │───►│ Operate   │───►│  Shutdown│
│           │    │   & Negotiate│    │           │    │          │
│  Open     │    │              │    │  Use      │    │  Close   │
│ Transport │    │  Capabilities│    │  Features │    │  Clean   │
└───────────┘    └──────────────┘    └───────────┘    └──────────┘
```

### Phase 1: Connection

The client establishes a transport connection to the MCP server.

**stdio:**

```go
cmd := exec.Command(server.Command, server.Args...)
cmd.Stdin = conn.stdin
cmd.Stdout = conn.stdout
cmd.Stderr = conn.stderr
err := cmd.Start()
```

**SSE:**

```go
// Connect to SSE endpoint for receiving messages
sseConn := http.Client{}.Get(server.SSEURL)
// Use messages endpoint for sending requests
msgURL := server.MessageEndpoint
```

### Phase 2: Initialization & Capability Negotiation

After transport is established, the client sends an `initialize` request.

#### Initialize Request

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2024-11-05",
    "capabilities": {
      "tools": {},
      "resources": {},
      "prompts": {}
    },
    "clientInfo": {
      "name": "coding-agent",
      "version": "0.1.0"
    }
  }
}
```

#### Initialize Response

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": "2024-11-05",
    "capabilities": {
      "tools": {
        "listChanged": true
      },
      "resources": {
        "subscribe": true,
        "listChanged": true
      },
      "prompts": {
        "listChanged": false
      }
    },
    "serverInfo": {
      "name": "filesystem-server",
      "version": "0.5.0"
    }
  }
}
```

#### Initialization Flow

```
Client                          Server
  │                              │
  │──── initialize ─────────────►│
  │                              │
  │◄──── InitializeResult ───────│
  │   (protocol version,         │
  │    capabilities, info)       │
  │                              │
  │──── initialized ────────────►│  (notification, no response)
  │                              │
  │──── tools/list ─────────────►│  (if tools capability)
  │                              │
  │◄──── ToolsListResult ────────│
  │   (available tools)          │
  │                              │
  │──── resources/list ─────────►│  (if resources capability)
  │                              │
  │◄──── ResourcesListResult ────│
  │   (available resources)      │
  │                              │
  │──── prompts/list ───────────►│  (if prompts capability)
  │                              │
  │◄──── PromptsListResult ──────│
  │   (available prompts)        │
  │                              │
```

#### Initialized Notification

After receiving the `initialize` response, the client MUST send an `initialized` notification before any other requests:

```json
{
  "jsonrpc": "2.0",
  "method": "notifications/initialized"
}
```

### Phase 3: Operation

During the operation phase, the client can:

- List and call tools
- List and read resources
- List and get prompts
- Receive server notifications

### Phase 4: Shutdown

Graceful shutdown involves:

```
Client                          Server
  │                              │
  │──── ping ───────────────────►│  (optional health check)
  │                              │
  │◄──── Pong ───────────────────│
  │                              │
  │  Close Transport             │
  │  (stdio: kill process)       │
  │  (SSE: close connection)     │
  │                              │
```

For stdio, the client should:

1. Send any pending requests
2. Close stdin
3. Wait for process exit (with timeout)
4. Force kill if timeout exceeded

## API Reference

### Tools

Tools allow the MCP server to expose callable functions to the agent.

#### List Tools

Retrieve available tools from the server.

**Request:**

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/list",
  "params": {
    "cursor": null // Optional pagination cursor
  }
}
```

**Response:**

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "tools": [
      {
        "name": "query_database",
        "description": "Execute a query against the database",
        "inputSchema": {
          "type": "object",
          "properties": {
            "table": {
              "type": "string",
              "description": "The table to query"
            },
            "filters": {
              "type": "object",
              "description": "Filter criteria for the query"
            },
            "limit": {
              "type": "integer",
              "description": "Maximum number of results",
              "default": 100
            }
          },
          "required": ["table"]
        }
      },
      {
        "name": "get_records",
        "description": "Retrieve records by ID",
        "inputSchema": {
          "type": "object",
          "properties": {
            "ids": {
              "type": "array",
              "items": { "type": "string" },
              "description": "Array of record IDs"
            }
          },
          "required": ["ids"]
        }
      }
    ],
    "nextCursor": null
  }
}
```

#### Call Tool

Execute a tool on the MCP server.

**Request:**

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "query_database",
    "arguments": {
      "table": "users",
      "filters": {
        "status": "active"
      },
      "limit": 10
    }
  }
}
```

**Response (Success):**

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "[\n  {\"id\": 1, \"name\": \"Alice\", \"status\": \"active\"},\n  {\"id\": 2, \"name\": \"Bob\", \"status\": \"active\"}\n]"
      }
    ],
    "isError": false
  }
}
```

**Response (Error):**

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Error: Table 'users' does not exist"
      }
    ],
    "isError": true
  }
}
```

#### Tool Content Types

Tool responses can contain different content types:

| Content Type | Description         | Example                 |
| ------------ | ------------------- | ----------------------- |
| `text`       | Plain text content  | Query results, messages |
| `image`      | Image data (base64) | Charts, screenshots     |
| `resource`   | Embedded resource   | File contents, URIs     |

```typescript
interface ContentBlock {
  type: "text" | "image" | "resource";
  text?: string; // For text type
  data?: string; // For image type (base64)
  mimeType?: string; // For image type
  resource?: EmbeddedResource; // For resource type
}

interface EmbeddedResource {
  uri: string;
  mimeType?: string;
  text?: string;
  blob?: string; // base64 encoded
}
```

### Resources

Resources provide read-only access to contextual data.

#### List Resources

**Request:**

```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "resources/list",
  "params": {}
}
```

**Response:**

```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "result": {
    "resources": [
      {
        "uri": "db://users/schema",
        "name": "Users Table Schema",
        "description": "Schema definition for the users table",
        "mimeType": "application/json"
      },
      {
        "uri": "db://config",
        "name": "Database Configuration",
        "description": "Current database configuration",
        "mimeType": "text/plain"
      }
    ]
  }
}
```

#### Read Resource

**Request:**

```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "method": "resources/read",
  "params": {
    "uri": "db://users/schema"
  }
}
```

**Response:**

```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "result": {
    "contents": [
      {
        "uri": "db://users/schema",
        "mimeType": "application/json",
        "text": "{\n  \"columns\": [\n    {\"name\": \"id\", \"type\": \"serial\"},\n    {\"name\": \"name\", \"type\": \"varchar(255)\"}\n  ]\n}"
      }
    ]
  }
}
```

#### Resource Templates

Some resources support parameterized URIs via templates.

**Template Response:**

```json
{
  "result": {
    "resourceTemplates": [
      {
        "uriTemplate": "db://tables/{table_name}/schema",
        "name": "Table Schema",
        "description": "Schema for any table",
        "mimeType": "application/json"
      }
    ]
  }
}
```

### Prompts

Prompts provide pre-defined prompt templates.

#### List Prompts

**Request:**

```json
{
  "jsonrpc": "2.0",
  "id": 6,
  "method": "prompts/list",
  "params": {}
}
```

**Response:**

```json
{
  "jsonrpc": "2.0",
  "id": 6,
  "result": {
    "prompts": [
      {
        "name": "review_code",
        "description": "Review code for best practices",
        "arguments": [
          {
            "name": "language",
            "description": "Programming language",
            "required": true
          },
          {
            "name": "focus",
            "description": "Review focus area (security, performance, style)",
            "required": false
          }
        ]
      }
    ]
  }
}
```

#### Get Prompt

**Request:**

```json
{
  "jsonrpc": "2.0",
  "id": 7,
  "method": "prompts/get",
  "params": {
    "name": "review_code",
    "arguments": {
      "language": "Go",
      "focus": "security"
    }
  }
}
```

**Response:**

```json
{
  "jsonrpc": "2.0",
  "id": 7,
  "result": {
    "messages": [
      {
        "role": "user",
        "content": {
          "type": "text",
          "text": "Please review the following Go code for security vulnerabilities. Focus on: authentication, input validation, and data exposure."
        }
      }
    ]
  }
}
```

### Sampling (Optional)

Some MCP servers support requesting LLM sampling from the client.

```json
{
  "jsonrpc": "2.0",
  "method": "sampling/createMessage",
  "params": {
    "messages": [
      {
        "role": "user",
        "content": {
          "type": "text",
          "text": "Summarize this data: ..."
        }
      }
    ],
    "modelPreferences": {
      "hints": [{ "name": "llama3" }],
      "costPriority": 0.5,
      "speedPriority": 0.5,
      "qualityPriority": 0.5
    },
    "maxTokens": 1000,
    "systemPrompt": "You are a helpful data analyst.",
    "includeContext": "none"
  }
}
```

## Calling Conventions

### Tool Calling Integration

MCP tools are integrated into the agent's tool calling system seamlessly.

#### Tool Registration

When an MCP server is initialized, its tools are registered with the agent's tool system:

```
MCP Tool Name:        query_database
Agent Tool Name:      mcp_filesystem_query_database
Prefix Convention:    mcp_{server_name}_{tool_name}
```

The prefix ensures unique tool names when multiple MCP servers are configured.

#### Tool Definition Mapping

MCP tool definitions are mapped to the agent's tool format:

```typescript
// MCP Tool Definition
{
  name: "query_database",
  description: "Execute a query against the database",
  inputSchema: {
    type: "object",
    properties: {
      table: { type: "string", description: "The table to query" },
      limit: { type: "integer", description: "Max results", default: 100 }
    },
    required: ["table"]
  }
}

// Mapped to Agent Tool Format
{
  type: "function",
  function: {
    name: "mcp_database_query_database",
    description: "[MCP: database] Execute a query against the database",
    parameters: {
      type: "object",
      properties: {
        table: { type: "string", description: "The table to query" },
        limit: { type: "integer", description: "Max results" }
      },
      required: ["table"]
    }
  }
}
```

#### Tool Execution Flow

```
1. LLM generates tool call: mcp_database_query_database({table: "users"})
2. Agent receives tool call and identifies MCP prefix
3. Agent routes to MCP Client Manager
4. Client Manager sends tools/call to appropriate MCP server
5. MCP server executes and returns result
6. Client Manager returns result to Agent
7. Agent formats result and sends to LLM
```

### Error Handling in Tool Calls

#### MCP Server Error

```json
// MCP server returns error result
{
  "content": [{"type": "text", "text": "Database connection failed"}],
  "isError": true
}

// Agent presents to LLM as tool result
{
  "tool_call_id": "call_abc123",
  "content": "Error executing mcp_database_query_database: Database connection failed",
  "isError": true
}
```

#### Transport Error

```json
// Connection lost, timeout, etc.
{
  "tool_call_id": "call_abc123",
  "content": "Error executing mcp_database_query_database: Connection to MCP server 'database' failed: timeout after 30s",
  "isError": true
}
```

### Result Formatting

MCP tool results are formatted for the LLM:

```typescript
function formatMcpResult(
  serverName: string,
  toolName: string,
  result: McpToolResult,
): string {
  const prefix = result.isError
    ? `Error from [${serverName}/${toolName}]: `
    : `Result from [${serverName}/${toolName}]: `;

  const contentBlocks = result.content.filter((block) => block.type === "text");
  const textContent = contentBlocks.map((block) => block.text).join("\n\n");

  return prefix + textContent;
}
```

## Configuration

### MCP Server Configuration

MCP servers are configured via environment variables or config file.

#### Config File Format

```ini
# Main configuration
api_endpoint=http://localhost:8080
model=llama3

# MCP Server 1: File System
[mcp_server.filesystem]
enabled=true
transport=stdio
command=npx
args=-y @modelcontextprotocol/server-filesystem /home/user/projects
timeout=30

# MCP Server 2: Database
[mcp_server.database]
enabled=true
transport=sse
url=http://localhost:3001/sse
endpoint=http://localhost:3001/messages
timeout=60
env.DATABASE_URL=postgres://user:pass@localhost/mydb

# MCP Server 3: API Gateway
[mcp_server.api_gateway]
enabled=true
transport=stdio
command=/usr/local/bin/api-mcp-server
args=--config /etc/mcp/api.json
timeout=30
```

#### Environment Variables

```bash
# MCP Server configuration via environment variables
export CODING_AGENT_MCP_FILESYSTEM_ENABLED=true
export CODING_AGENT_MCP_FILESYSTEM_TRANSPORT=stdio
export CODING_AGENT_MCP_FILESYSTEM_COMMAND=npx
export CODING_AGENT_MCP_FILESYSTEM_ARGS="-y @modelcontextprotocol/server-filesystem /home/user/projects"
export CODING_AGENT_MCP_FILESYSTEM_TIMEOUT=30

# Environment variables passed to MCP server process
export CODING_AGENT_MCP_FILESYSTEM_ENV_DATABSE_URL=postgres://user:pass@localhost/mydb
```

### Configuration Schema

```typescript
interface McpServerConfig {
  // Server identification
  name: string; // Unique server name (used in tool prefix)

  // Transport configuration
  transport: "stdio" | "sse"; // Transport type

  // stdio transport options
  command?: string; // Command to execute
  args?: string[]; // Command arguments
  cwd?: string; // Working directory

  // SSE transport options
  url?: string; // SSE endpoint URL
  endpoint?: string; // Message endpoint URL
  headers?: Record<string, string>; // Custom HTTP headers

  // Common options
  enabled?: boolean; // Whether this server is enabled
  timeout?: number; // Request timeout in seconds
  reconnect?: boolean; // Auto-reconnect on failure
  maxRetries?: number; // Maximum reconnection attempts
  env?: Record<string, string>; // Environment variables for subprocess
}

interface McpClientConfig {
  servers: Record<string, McpServerConfig>;
}
```

## Reconnection Policy

### Automatic Reconnection

When a transport error occurs, the client can attempt to reconnect:

```
Connection Lost
     │
     ▼
┌─────────────┐     Yes      ┌───────────────┐
│ Retry count │─────────────►│ Wait (backoff)│
│ < max?      │              └──────┬────────┘
└──────┬──────┘                     │
       │ No                         ▼
       │                  ┌────────────────┐
       ▼                  │ Reconnect &    │
┌──────────────┐          │ Re-initialize  │
│ Log Error &  │          └────────────────┘
│ Disconnect   │                     │
└──────────────┘                     ▼
                            ┌──────────────┐
                            │ Resume Tools │
                            └──────────────┘
```

### Backoff Strategy

Exponential backoff with jitter:

```
delay = min(base_delay * 2^attempt + random_jitter, max_delay)
```

Default values:

- `base_delay`: 1 second
- `max_delay`: 30 seconds
- `max_attempts`: 5

## Notification Handling

### Server Notifications

The client handles the following server notifications:

#### tools/listChanged

```json
{
  "jsonrpc": "2.0",
  "method": "notifications/tools/list_changed"
}
```

**Client Action:** Re-fetch tool list and update registered tools.

#### resources/listChanged

```json
{
  "jsonrpc": "2.0",
  "method": "notifications/resources/list_changed"
}
```

**Client Action:** Re-fetch resource list.

#### progress

```json
{
  "jsonrpc": "2.0",
  "method": "notifications/progress",
  "params": {
    "progressToken": "abc123",
    "progress": 50,
    "total": 100
  }
}
```

**Client Action:** Update progress display in TUI.

## Integration with Agent Features

### Context Size Management

MCP tool results contribute to context size:

```
Total Context = System Prompt + User Messages + MCP Tool Results + Built-in Tool Results
```

Large MCP tool results should be truncated or summarized:

```typescript
const MAX_MCP_RESULT_SIZE = 8192; // characters

function truncateMcpResult(result: string): string {
  if (result.length <= MAX_MCP_RESULT_SIZE) return result;

  const truncated = result.substring(0, MAX_MCP_RESULT_SIZE);
  return (
    truncated +
    `\n\n[Note: Response truncated. ${result.length} total characters, showing first ${MAX_MCP_RESULT_SIZE}.]`
  );
}
```

### Read-Only Mode

In read-only mode, MCP tools should be evaluated for safety:

```typescript
interface McpToolCapability {
  readOnly?: boolean; // true if tool only reads data
  destructive?: boolean; // true if tool modifies/deletes data
}
```

MCP servers can declare tool capabilities. In read-only mode, only tools marked `readOnly: true` are available.

### Debug Logging

When debug mode is enabled, MCP communication is logged:

```
[DEBUG] [MCP:filesystem] Sending request: {"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}
[DEBUG] [MCP:filesystem] Received response: {"jsonrpc":"2.0","id":1,"result":{"tools":[...]}}
[DEBUG] [MCP:filesystem] Calling tool: query_database with args {"table":"users"}
[DEBUG] [MCP:filesystem] Tool result (isError=false): [{"type":"text","text":"..."}]
```

## Go Type Definitions

### Core Types

```go
package mcp

// McpRequest represents a JSON-RPC request
type McpRequest struct {
    JSONRPC string `json:"jsonrpc"`
    ID      int    `json:"id"`
    Method  string `json:"method"`
    Params  *json.RawMessage `json:"params,omitempty"`
}

// McpResponse represents a JSON-RPC response
type McpResponse struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      int             `json:"id"`
    Result  *json.RawMessage `json:"result,omitempty"`
    Error   *McpError        `json:"error,omitempty"`
}

// McpError represents a JSON-RPC error
type McpError struct {
    Code    int              `json:"code"`
    Message string           `json:"message"`
    Data    *json.RawMessage `json:"data,omitempty"`
}

// McpNotification represents a JSON-RPC notification (no ID)
type McpNotification struct {
    JSONRPC string `json:"jsonrpc"`
    Method  string `json:"method"`
    Params  *json.RawMessage `json:"params,omitempty"`
}
```

### Tool Types

```go
package mcp

// Tool represents an MCP tool
type Tool struct {
    Name        string          `json:"name"`
    Description string          `json:"description"`
    InputSchema json.RawMessage `json:"inputSchema"`
}

// CallToolRequest is the params for tools/call
type CallToolRequest struct {
    Name      string          `json:"name"`
    Arguments json.RawMessage `json:"arguments,omitempty"`
}

// CallToolResult is the result of a tools/call
type CallToolResult struct {
    Content []ContentBlock `json:"content"`
    IsError bool           `json:"isError,omitempty"`
}

// ContentBlock represents a piece of content in a result
type ContentBlock struct {
    Type     string           `json:"type"`
    Text     string           `json:"text,omitempty"`
    Data     string           `json:"data,omitempty"`
    MimeType string           `json:"mimeType,omitempty"`
    Resource *EmbeddedResource `json:"resource,omitempty"`
}

// EmbeddedResource represents an embedded resource
type EmbeddedResource struct {
    URI      string `json:"uri"`
    MimeType string `json:"mimeType,omitempty"`
    Text     string `json:"text,omitempty"`
    Blob     string `json:"blob,omitempty"`
}
```

### Connection Types

```go
package mcp

// McpConnection manages a connection to an MCP server
type McpConnection struct {
    Config       McpServerConfig
    Transport    Transport
    Capabilities ServerCapabilities
    ServerInfo   ServerInfo
    Tools        []Tool
    Resources    []Resource
    Prompts      []Prompt
    isInitialized bool
    mu           sync.Mutex
}

// Transport abstracts the communication channel
type Transport interface {
    Send(request McpRequest) (*McpResponse, error)
    SendNotification(notification McpNotification) error
    Receive() (McpMessage, error)
    Close() error
    IsConnected() bool
}

// StdioTransport implements Transport using stdio
type StdioTransport struct {
    Cmd    *exec.Cmd
    Stdin  *io.PipeWriter
    Stdout *io.PipeReader
    Stderr io.Writer
}

// SSETransport implements Transport using Server-Sent Events
type SSETransport struct {
    HTTPClient *http.Client
    SSEURL     string
    MsgURL     string
    Headers    map[string]string
    SSEConn    *SSEConnection
}
```

### Client Manager

```go
package mcp

// McpClientManager manages multiple MCP server connections
type McpClientManager struct {
    Config     McpClientConfig
    Connections map[string]*McpConnection
    mu         sync.RWMutex
}

// NewMcpClientManager creates a new MCP client manager
func NewMcpClientManager(config McpClientConfig) *McpClientManager

// Initialize all configured MCP servers
func (m *McpClientManager) Initialize() error

// Shutdown all MCP server connections
func (m *McpClientManager) Shutdown() error

// GetTool executes a tool call on the appropriate MCP server
func (m *McpClientManager) GetTool(serverName, toolName string, arguments map[string]interface{}) (*CallToolResult, error)

// ListTools returns all tools from all connected MCP servers
func (m *McpClientManager) ListTools() []Tool

// GetConnection returns a specific server connection
func (m *McpClientManager) GetConnection(name string) *McpConnection

// IsConnected checks if a server is currently connected
func (m *McpClientManager) IsConnected(name string) bool
```

## Error Codes

MCP uses standard JSON-RPC error codes:

| Code                 | Name             | Description                |
| -------------------- | ---------------- | -------------------------- |
| `-32700`             | Parse Error      | Invalid JSON               |
| `-32600`             | Invalid Request  | Invalid request structure  |
| `-32601`             | Method Not Found | Unknown method             |
| `-32602`             | Invalid Params   | Invalid method parameters  |
| `-32603`             | Internal Error   | Internal server error      |
| `-32000` to `-32099` | Server Error     | Server-defined error codes |

### Server-Specific Errors

| Code     | Description        |
| -------- | ------------------ |
| `-32001` | Connection closed  |
| `-32002` | Request timeout    |
| `-32003` | Content too large  |
| `-32004` | Resource not found |
| `-32005` | Tool not found     |
| `-32006` | Invalid URI        |

## Security Considerations

### Sandboxing

MCP servers may execute arbitrary code. Consider:

- Running MCP servers in containers
- Limiting filesystem access
- Restricting network access
- Running with limited privileges

### Input Validation

- Validate all tool arguments against the declared schema
- Limit input size to prevent DoS attacks
- Sanitize file paths to prevent directory traversal

### Authentication

For SSE transport, support authentication via:

- Bearer tokens in headers
- API keys in headers
- mTLS certificates

```ini
[mcp_server.secure_service]
transport=sse
url=https://secure.example.com/sse
headers.Authorization=Bearer eyJhbGc...
headers.X-API-Key=your-api-key
```

## Example Integrations

### Example 1: File System Server

```ini
[mcp_server.filesystem]
enabled=true
transport=stdio
command=npx
args=-y @modelcontextprotocol/server-filesystem /home/user/projects
timeout=30
```

Available tools:

- `mcp_filesystem_read_file` - Read a file's contents
- `mcp_filesystem_write_file` - Write to a file
- `mcp_filesystem_list_directory` - List directory contents
- `mcp_filesystem_search_files` - Search for files by pattern

### Example 2: PostgreSQL Database

```ini
[mcp_server.postgres]
enabled=true
transport=stdio
command=npx
args=-y @modelcontextprotocol/server-postgres
env.DATABASE_URL=postgres://user:password@localhost:5432/mydb
timeout=30
```

Available tools:

- `mcp_postgres_query` - Execute a SQL query
- `mcp_postgres_list_tables` - List all tables
- `mcp_postgres_describe_table` - Describe table schema

Resources:

- `db://tables/{table}/schema` - Table schema
- `db://config` - Database configuration

### Example 3: Git Server

```ini
[mcp_server.git]
enabled=true
transport=stdio
command=npx
args=-y @modelcontextprotocol/server-git
timeout=30
```

Available tools:

- `mcp_git_log` - View commit history
- `mcp_git_diff` - View diff between commits
- `mcp_git_status` - View current status
- `mcp_git_branch` - List/create branches

## Acceptance Criteria

- [ ] MCP client can connect to servers via stdio transport
- [ ] MCP client can connect to servers via SSE transport
- [ ] Initialization handshake completes successfully (initialize → initialized)
- [ ] Tool listing works for all connected servers
- [ ] Tool calls are routed to the correct server
- [ ] Tool results are properly formatted for the LLM
- [ ] Multiple MCP servers can be configured simultaneously
- [ ] Tool names are prefixed with server name to ensure uniqueness
- [ ] Connection errors are handled gracefully
- [ ] Automatic reconnection is supported with exponential backoff
- [ ] Server notifications (tools/listChanged, resources/listChanged) are handled
- [ ] MCP configuration via config file and environment variables
- [ ] MCP tools are included in the system prompt sent to the LLM
- [ ] MCP tool results respect context size limits
- [ ] Read-only mode filters MCP tools appropriately
- [ ] Debug logging captures MCP communication
- [ ] Timeout handling for long-running tool calls
- [ ] Graceful shutdown closes all MCP connections

## References

- [Model Context Protocol Specification](https://modelcontextprotocol.io/specification)
- [MCP GitHub Repository](https://github.com/modelcontextprotocol/spec)
- [JSON-RPC 2.0 Specification](https://www.jsonrpc.org/specification)
- [Inference API Specification](./inference-api.md)
