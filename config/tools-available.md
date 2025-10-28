# Tools Available for TmuxAI Personas

The following non-standard tools are installed and may be used when appropriate. When a tool is unavailable, fall back to core POSIX utilities.

## Search & Navigation

- `rg` *(ripgrep)* – project-wide regex search respecting `.gitignore`.
- `fd` – intuitive alternative to `find`; supports globbing and excludes.
- `exa` – enhanced directory listings with tree mode.

## Editing & Formatting

- `nvim` – modal editor; automation via `nvim --headless` and `:lua` scripts.

## Data Processing & Scripting

- `jq` – JSON query and transformation.
- `xh` – http client (curl alternative) with JSON processing.
- `awk` – pattern scanning and text processing (POSIX awk implementation).
- `sed` – stream editor for substitution, filtering, and transformations.
- `sd` – modern drop-in replacement for `sed` with clearer syntax.
- `ssed` – enhanced sed supporting in-place edits and backups.
- `locate` – fast file lookup using pre-built database; refresh via `updatedb` if needed.

## Build & Package

- `pnpm` – Node.js package manager with workspace support.
- `pipx` – install/run isolated Python CLIs.
- `cargo` – Rust package manager and build tool.
- `go` – Go toolchain for builds and installs.
- `python` – CPython runtime and package scripts.
- `node` – Node.js runtime for JS/TS workflows.
- `rustc` – Rust compiler available alongside `cargo`.

## Testing & QA

- `pytest` – Python testing (when project includes it).

## Observability & Ops

- `kubectl` – Kubernetes management (non-interactive subcommands preferred).

## Git & Release

- Core Git toolchain available via `git`.

## Container & Virtualization

- `podman` – rootless container engine with Docker-compatible CLI (use `podman compose`).

## Security & Compliance

- (Install additional scanners as needed.)

## Release Engineering Helpers

- Extend with release automation tooling when installed.

## Utilities

- `tldr` – concise command examples (cacheable offline).

## Environment

- Shell: `fish`
- Multiplexer: `tmux`
- Container runtime: Podman (Docker CLI compatible via aliases).

> Keep commands concise. Confirm tool availability with `command -v <name>` if uncertain.
