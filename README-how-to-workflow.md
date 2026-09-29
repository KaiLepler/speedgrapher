# Speedgrapher Workflow & Integration Guide

This guide details how to build, run, and integrate Speedgrapher into your local coding and writing environments, specifically **Antigravity (AGY)** and **Warp**.

---

## 1. Quick Build & Installation

From inside the Speedgrapher repository (`/Users/kai/Developer/GitHub/speedgrapher`):

```bash
# 1. Compile the local binary (creates bin/speedgrapher)
make build

# 2. (Recommended) Copy binary to ~/.local/bin so it's on your shell PATH for all terminals
cp bin/speedgrapher ~/.local/bin/
```

Verify that the binary runs:
```bash
speedgrapher version
speedgrapher check
```

---

## 2. Antigravity (AGY) Workflow

Speedgrapher is natively optimized for Antigravity and the Gemini CLI ecosystem.

### Step 1: One-Time Global Installation
Run the installer once from this repository:

```bash
./bin/speedgrapher install
```

#### What happens behind the scenes:
1. **MCP Server Registration**: Registers `speedgrapher` in `~/.gemini/config/mcp_config.json`:
   ```json
   {
     "mcpServers": {
       "speedgrapher": {
         "command": "/Users/kai/Developer/GitHub/speedgrapher/bin/speedgrapher",
         "args": ["mcp"]
       }
     }
   }
   ```
2. **Unpacks Editorial Skills**: Installs 6 specialized writing skills into `~/.gemini/config/skills/`:
   * `@deslopify` — Strips AI clichés, filler buzzwords, and repetitive syntactic tropes.
   * `@inverted-pyramid` — Enforces high-impact information cascading.
   * `@tech-interviewer` — Extracts problem statements and logs before drafting.
   * `@tech-writer` — Structured technical writing and tone alignment.
   * `@tech-reviewer` — Quality gate analyzing drafts against `fog`, `slop`, and `vale`.
   * `@tech-publisher` — Pre-publication SEO checklists and metadata audit.

### Step 2: Using Across Any Project in AGY
Because `~/.gemini/config` is Antigravity's **Global Customizations Root**, everything installed there is **automatically inherited across all workspaces and projects** (e.g. your Modern PPT Creator, blog repositories, documentation folders, or CLI apps).

**You do NOT need to add Speedgrapher to your other project repos.**

Simply open your other project in AGY and use natural language:

* **Checking for AI clichés and buzzwords:**
  > *"Analyze slide 2 using the slop tool to identify any overused AI buzzwords."*  
  > *(AGY invokes the `slop` MCP tool in the background)*

* **Checking readability level:**
  > *"What is the Gunning Fog score for the introduction paragraph in `README.md`?"*  
  > *(AGY invokes the `fog` MCP tool)*

* **Using the writing personas:**
  > *"Use @deslopify to rewrite the copy in `intro.md` to sound natural and direct."*  
  > *(AGY activates the embedded `@deslopify` skill)*

---

## 3. Warp Workflow

In Warp, Speedgrapher can be used in two complementary ways: **Direct CLI / Pipes** in the terminal, and **Native MCP Integration** in Warp AI Agent.

### Option A: CLI Commands & Pipes in Warp Terminal
Because `~/.local/bin` is in your shell `$PATH`, `speedgrapher` is available anywhere in Warp without needing to switch directories:

```bash
# 1. Quick ad-hoc text check
speedgrapher call slop '{"text": "In today'\''s fast-paced digital landscape, delve into the intricate tapestry of AI."}'

# 2. Pipe files directly into tools
cat slide.md | speedgrapher call slop
cat draft.md | speedgrapher call fog

# 3. Check technical SEO of an HTML file or Hugo markdown
speedgrapher call seo '{"path": "index.html", "keyword": "speedgrapher"}'
```

#### Using with Warp AI (Cmd + K / Terminal Prompts)
You can ask Warp's terminal assistant to leverage the CLI:
> *"Run `cat slide.md | speedgrapher call slop` and rewrite lines with high slop scores."*

---

### Option B: Warp AI Agent (Native MCP Support)
Warp supports the Model Context Protocol natively. You can connect Speedgrapher as an MCP server so Warp's AI Agent can invoke editorial tools directly during chat:

1. Open Warp **Settings** (`Cmd + ,`).
2. Go to **AI** $\rightarrow$ **MCP Servers** (or edit `~/.warp/mcp.json`).
3. Click **Add Server**:
   * **Name**: `speedgrapher`
   * **Command**: `/Users/kai/Developer/GitHub/speedgrapher/bin/speedgrapher` (or `speedgrapher` if copied to `~/.local/bin`)
   * **Args**: `["mcp"]`
4. In Warp's AI Agent panel, the agent will now list `fog`, `slop`, `analyze_seo`, and `vale` as available tools.

---

## 4. MCP Transport Modes: Stdio vs. Streamable HTTP

Speedgrapher provides two transport protocols depending on your architectural needs:

| Feature | Stdio Mode (`stdio`) | Streamable HTTP Mode (`http`) |
| :--- | :--- | :--- |
| **Execution** | Spawned as a child process by the client | Long-running background daemon |
| **Communication** | `stdin` / `stdout` process pipes | HTTP POST with Server-Sent Events (SSE) streaming |
| **Process Lifecycle** | Controlled by the IDE/agent automatically | Managed externally (systemd, background, Docker) |
| **Port / Networking** | No open ports (local process only) | Listens on TCP port (e.g. `:8080`) with CORS |
| **Best For** | Local coding clients (Antigravity, Warp, Cursor) | Centralized team services, web dashboards, Docker, CI/CD |

### Running Streamable HTTP
If you ever want to run Speedgrapher as an independent microservice for web apps or remote agents:

```bash
# Start server listening on port 8080
speedgrapher mcp --listen=:8080
```

#### Calling via `curl` / HTTP:
```bash
# List tools
curl -s -X POST http://localhost:8080 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc": "2.0", "id": 1, "method": "tools/list", "params": {}}'

# Call tool
curl -s -X POST http://localhost:8080 \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/call",
    "params": {
      "name": "slop",
      "arguments": {
        "text": "Delve into the intricate tapestry of modern software."
      }
    }
  }'
```

---

## 5. Version Bumping & Release Workflow

### How Versioning Works
Speedgrapher has **no hardcoded version strings** in code. The Git tag is the single source of truth (ADR-0005):
* When building (`make build`), the Makefile automatically executes `git describe --tags` and injects that version into the binary using Go's linker flags (`-ldflags "-X main.version=..."`).

### Order of Operations: Tag FIRST, then Build
Because the binary gets its version dynamically from Git tags:

1. **Test changes:**
   ```bash
   make test
   ```
2. **Tag the commit first:**
   ```bash
   make bump-version VERSION=0.10.1
   ```
   *(This creates local git tag `v0.10.1`)*
3. **Compile the binary stamped with the new version:**
   ```bash
   make build
   ./bin/speedgrapher version # prints 0.10.1
   ```
4. **Push the tag to your remote repository:**
   ```bash
   git push origin v0.10.1
   ```

*(Tip: For quick local development without creating a Git tag, you can force any version directly using `make build VERSION=0.10.1`).*

