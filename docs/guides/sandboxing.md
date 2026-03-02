# Sandboxing

OmniAgent provides layered security for tool execution through capability-based permissions and runtime isolation.

## Overview

Tools can be sandboxed at multiple levels:

1. **App-Level Permissions** - Capability-based access control
2. **WASM Isolation** - Lightweight sandboxing via wazero
3. **Docker Isolation** - Full container isolation

## App-Level Permissions

### Capabilities

Tools request specific capabilities:

| Capability | Description |
|------------|-------------|
| `fs_read` | Read files from allowed paths |
| `fs_write` | Write files to allowed paths |
| `net_http` | Make HTTP requests to allowed hosts |
| `exec_run` | Execute allowed commands |

### Configuration

```go
config := sandbox.Config{
    Capabilities: []sandbox.Capability{
        sandbox.CapFSRead,
        sandbox.CapNetHTTP,
    },
    AllowedPaths: []string{"/tmp/data", "/home/user/docs"},
    AllowedHosts: []string{"api.example.com"},
    AllowedCommands: []string{"ls", "cat"},
}
```

## WASM Sandbox

For lightweight isolation, tools can run in a WASM sandbox using [wazero](https://github.com/tetratelabs/wazero).

### Features

- Memory limits
- Timeout enforcement
- No network access by default
- Restricted file system access

### Usage

```go
runtime, err := sandbox.NewRuntime(ctx, sandbox.Config{
    Capabilities:  []sandbox.Capability{sandbox.CapFSRead},
    MemoryLimitMB: 16,
    Timeout:       30 * time.Second,
    AllowedPaths:  []string{"/tmp/data"},
})

result, err := runtime.Run(ctx, wasmModule, args)
```

### Host Functions

The WASM sandbox exposes controlled host functions:

| Function | Capability Required | Description |
|----------|---------------------|-------------|
| `fs_read` | `CapFSRead` | Read file contents |
| `fs_write` | `CapFSWrite` | Write file contents |
| `http_fetch` | `CapNetHTTP` | Make HTTP requests |
| `exec_run` | `CapExecRun` | Execute commands |

## Docker Sandbox

For OS-level isolation, tools can run inside Docker containers.

### Features

- Full process isolation
- Network restrictions
- Capability dropping
- Read-only mounts

### Usage

```go
sandbox, err := sandbox.NewDockerSandbox(ctx, sandbox.DockerConfig{
    Image:       "alpine:latest",
    NetworkMode: "none",           // No network access
    CapDrop:     []string{"ALL"},  // Drop all capabilities
    Mounts: []sandbox.DockerMount{
        {
            HostPath:      "/tmp/data",
            ContainerPath: "/data",
            ReadOnly:      true,
        },
    },
}, &appConfig)

result, err := sandbox.Run(ctx, "cat", []string{"/data/file.txt"})
```

### Network Modes

| Mode | Description |
|------|-------------|
| `none` | No network access |
| `bridge` | Isolated network with NAT |
| `host` | Full host network access (not recommended) |

### Security Hardening

```go
config := sandbox.DockerConfig{
    Image:       "alpine:latest",
    NetworkMode: "none",
    CapDrop:     []string{"ALL"},
    ReadOnlyRootfs: true,
    NoNewPrivileges: true,
}
```

## Best Practices

### Principle of Least Privilege

Only grant the minimum capabilities required:

```go
// Bad: Too permissive
config := sandbox.Config{
    Capabilities: []sandbox.Capability{
        sandbox.CapFSRead,
        sandbox.CapFSWrite,
        sandbox.CapNetHTTP,
        sandbox.CapExecRun,
    },
}

// Good: Minimal permissions
config := sandbox.Config{
    Capabilities: []sandbox.Capability{sandbox.CapFSRead},
    AllowedPaths: []string{"/tmp/readonly"},
}
```

### Path Restrictions

Always restrict file access to specific directories:

```go
config := sandbox.Config{
    AllowedPaths: []string{
        "/tmp/workspace",
        "/home/user/data",
    },
}
```

### Command Allowlists

Only allow specific commands for exec:

```go
config := sandbox.Config{
    AllowedCommands: []string{"ls", "cat", "grep"},
}
```

### Timeouts

Always set reasonable timeouts:

```go
config := sandbox.Config{
    Timeout: 30 * time.Second,
}
```

## Choosing a Sandbox

| Use Case | Recommended |
|----------|-------------|
| Simple file operations | App-level permissions |
| Untrusted code execution | WASM sandbox |
| Complex tools with dependencies | Docker sandbox |
| Maximum isolation | Docker with `none` network |
