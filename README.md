# output-monitor

Split-pane TUI that pipes stdout into two views: all logs on top, filtered matches on bottom.

```
dbt run 2>&1 | output-monitor ERROR WARNING
```

```
┌─ All Logs (142 lines) • 12/s ──────────────────┐
│ 15:04:01.123 INFO: starting run                 │
│ 15:04:02.456 ERROR: connection refused          │
│ 15:04:03.789 INFO: retrying...                  │
│ 15:04:05.012 WARNING: slow response             │
└─────────────────────────────────────────────────┘
┌─ Filtered: [ERROR, WARNING] -t -C1 (2 lines) ──┐
│ 15:04:01.900 INFO: retrying...          ← context│
│ 15:04:02.456 ERROR: connection refused  ← match  │
│ 15:04:03.100 INFO: retry succeeded      ← context│
│ ---                                             │
│ 15:04:04.800 INFO: slow query           ← context│
│ 15:04:05.012 WARNING: slow response     ← match  │
└─────────────────────────────────────────────────┘
q: quit • f: toggle follow • ↑↓/pgup/pgdn: scroll
```

## Install

### Nix flake (recommended)

Add to your flake inputs:

```nix
inputs.output-monitor.url = "github:rhousand/output-monitor";
```

**NixOS module:**
```nix
imports = [ inputs.output-monitor.nixosModules.default ];

programs.output-monitor.enable = true;
```

**Direct package:**
```nix
environment.systemPackages = [
  inputs.output-monitor.packages.${system}.default
];
```

### One-shot (no install)

```bash
some-command 2>&1 | nix run github:rhousand/output-monitor -- ERROR WARNING
```

### Build from source

```bash
git clone https://github.com/rhousand/output-monitor
cd output-monitor
go build -o output-monitor .
```

## Usage

```
output-monitor [flags] <term> [term...]
```

Pipe any command's stdout/stderr into `output-monitor` and pass one or more filter terms.
Matched lines appear highlighted in the bottom pane.

## Flags

| Flag | Description |
|------|-------------|
| `-i` | Case-insensitive matching |
| `-r` | Treat terms as regular expressions |
| `-t` | Prefix each line with a timestamp (`HH:MM:SS.mmm`) |
| `-C N` | Show N lines of context before and after each match |
| `-b` | Ring terminal bell on each match |
| `-o file` | Write all output to file (raw, no ANSI codes) |

## Examples

```bash
# Basic — show lines containing ERROR or WARNING
dbt run 2>&1 | output-monitor ERROR WARNING

# Case-insensitive
dbt run 2>&1 | output-monitor -i error warning

# Timestamps + 2 lines of context
dbt run 2>&1 | output-monitor -t -C 2 ERROR WARNING

# Regex — match ERROR or WARN anywhere
dbt run 2>&1 | output-monitor -r "ERROR|WARN"

# Save full log to file while monitoring
dbt run 2>&1 | output-monitor -o run.log ERROR WARNING

# Bell alert + timestamps for long-running jobs
dbt run 2>&1 | output-monitor -b -t ERROR
```

## Keybindings

| Key | Action |
|-----|--------|
| `q` / `Ctrl+C` | Quit |
| `f` | Toggle auto-follow (scroll lock) |
| `↑` / `↓` | Scroll active pane |
| `PgUp` / `PgDn` | Scroll active pane by page |
