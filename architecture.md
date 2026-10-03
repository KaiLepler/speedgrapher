# Speedgrapher Architecture

This document provides a high-level overview of the Speedgrapher system architecture, detailing the components, data flows, external connections, and sequence views for processing requests.

## Component Overview

Speedgrapher acts as a Model Context Protocol (MCP) server, integrating external static analysis and documentation tools into AI coding clients like AGY and Streamable.

```mermaid
flowchart TD
    %% Clients
    subgraph Clients["MCP Clients"]
        AGY["AGY / Warp\n(Local IDE)"]
        Streamable["Streamable Client\n(Web/Remote)"]
    end

    %% Speedgrapher Core
    subgraph Speedgrapher["Speedgrapher MCP Server"]
        CLI["CLI Entrypoint\n(cmd/speedgrapher)"]
        Server["MCP Server\n(internal/server)"]
        
        subgraph Transports["Transports"]
            Stdio["Stdio Handler"]
            HTTP["HTTP/SSE Handler"]
        end
        
        subgraph Tools["Tool Handlers"]
            ValeTool["Vale Tool\n(internal/tools/vale)"]
            SlopTool["Slop Tool"]
            SeoTool["SEO Tool"]
            FogTool["Fog Tool"]
        end
        
        SafeShell["SafeShell\n(internal/safeshell)"]
    end

    %% Local File System
    subgraph LocalSystem["Local Environment"]
        FS["File System\n(Workspace files, .vale.ini)"]
        Binaries["Local Binaries\n(vale.exe, etc.)"]
    end

    %% Internet Connections
    subgraph Internet["Internet / External Services"]
        GitHub["GitHub Releases\n(errata-ai/vale)"]
    end

    %% Connections
    AGY -- "JSON-RPC via Stdio" --> Stdio
    Streamable -- "JSON-RPC via HTTP/SSE" --> HTTP
    
    Stdio --> Server
    HTTP --> Server
    CLI --> Transports
    
    Server -- "Dispatches CallTool" --> Tools
    Tools -- "Executes commands" --> SafeShell
    
    SafeShell -- "Reads/Writes" --> FS
    SafeShell -- "Executes" --> Binaries
    
    %% Bootstrap processes
    ValeTool -. "Downloads binary\nif missing (HTTPS GET)" .-> GitHub
    ValeTool -. "Extracts & Caches" .-> Binaries
```

## Data Flows and Internet Connections

Speedgrapher is designed to be privacy-conscious and primarily operates locally. However, it does require some connections to the internet for bootstrapping dependencies.

**From Local Computer to the Internet:**
1. **Binary Bootstrapping (e.g., Vale):** When the server starts or a tool is invoked for the first time, the tool handler (e.g., `internal/tools/vale/bootstrap.go`) checks if the required underlying binary (like `vale`) is installed. If it is not found, Speedgrapher makes an `HTTPS GET` request to the internet (specifically `https://github.com/errata-ai/vale/releases/download/...`) to download the binary.
2. **Configuration Syncing:** Some tools may need to sync their styles or configurations (e.g., `vale sync`), which may also reach out to GitHub or configured package repositories to pull down style rules.

**Data Flow Summary:**
* **No Telemetry:** Speedgrapher itself does not send your code, prompts, or workspace data to the internet.
* **Local Processing:** All static analysis and tool execution happens strictly on your local machine via the `SafeShell` component. The tool receives the file content from the MCP client or reads it from the local filesystem, processes it through the local binary, and returns the JSON output back to the MCP client.

---

## Sequence Views

The following sequence diagrams illustrate how different use cases are processed within the Speedgrapher system.

### Use Case 1: Tool List Request (`tools/list`)

When the MCP client connects, it requests the list of available tools to understand what Speedgrapher can do.

```mermaid
sequenceDiagram
    participant Client as MCP Client (AGY)
    participant Server as Server (Transport)
    participant Core as MCP Core
    participant Tool as Tool Handlers

    Client->>Server: JSON-RPC Request (tools/list)
    Server->>Core: HandleRequest
    Core->>Tool: Retrieve registered tools
    Tool-->>Core: [Vale, Slop, SEO, Fog]
    Core-->>Server: JSON-RPC Response (List of tools & schemas)
    Server-->>Client: Return Tools List
```

### Use Case 2: Tool Call Execution (`tools/call`)

When the AI assistant decides to use a tool (e.g., run Vale on a markdown file).

```mermaid
sequenceDiagram
    participant Client as MCP Client
    participant Server as MCP Server
    participant Tool as Vale Tool Handler
    participant SafeShell as SafeShell
    participant FS as Local File System
    participant Internet as Internet (GitHub)

    Client->>Server: JSON-RPC Request (tools/call: vale, text="...")
    Server->>Tool: Invoke ValeHandler
    
    opt Bootstrap Missing Binary
        Tool->>Internet: HTTPS GET (Download Vale Release)
        Internet-->>Tool: Archive (tar.gz/zip)
        Tool->>FS: Extract and Cache Binary
    end
    
    Tool->>SafeShell: Execute (vale --config .vale.ini --output JSON)
    SafeShell->>FS: Spawn Process (vale)
    FS-->>SafeShell: Process Output (stdout/stderr)
    SafeShell-->>Tool: Combined Output
    
    Tool-->>Server: Parse & Format ToolResult
    Server-->>Client: JSON-RPC Response (Vale Results)
```

### Use Case 3: Process Cleanup (SafeShell Isolation)

To prevent zombie processes (a known issue), SafeShell ensures that any subprocesses spawned by tools are properly terminated, even if the MCP client cancels the request or disconnects.

```mermaid
sequenceDiagram
    participant Tool as Tool Handler
    participant SafeShell as SafeShell
    participant OS as Operating System
    participant Process as Subprocess (e.g., vale)

    Tool->>SafeShell: Execute(context, command)
    SafeShell->>OS: Setup Process Group (SysProcAttr)
    OS->>Process: Start Process in new Group ID
    
    alt Normal Completion
        Process-->>SafeShell: Exit Code 0
        SafeShell-->>Tool: Return Output
    else Context Canceled / Timeout
        SafeShell->>OS: Kill Process Group (-PID)
        OS->>Process: SIGKILL to entire group
        SafeShell-->>Tool: Return Error (Timeout/Canceled)
    end
```
