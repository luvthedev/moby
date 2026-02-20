---
version: "2.0"
---

# Intelligence

## Tech Stack
- **tech_stack**: Go, Docker, Cloud SDKs (AWS, Google Cloud, Azure), gRPC, Protocol Buffers
- **primary_language**: Go
- **detected_files**: go.mod, go.sum
- **rationale**: Repository is github.com/moby/moby (Docker), the primary container runtime. GitHub statistics show 97.3% Go code, confirming Go as the primary language. The go.mod file reveals extensive dependencies including cloud.google.com/go, aws/aws-sdk-go-v2, Microsoft/hcsshim (Windows container support), and protocol buffer tooling. The presence of Dockerfile (0.5%), Shell (1.4%), and PowerShell (0.3%) indicates multi-platform support (Linux, Windows). The dependency on cloud.google.com/go/logging, AWS SDK, and Azure libraries shows integration with major cloud platforms. This is a systems-level containerization project written in Go with multi-cloud support.
- **alternatives_rejected**: {'language': 'Python', 'reason': 'Only 0.1% Python code detected; no requirements.txt or pyproject.toml found'}, {'language': 'Rust', 'reason': 'No Cargo.toml or Rust dependencies detected; Go is the established language for this project'}, {'language': 'C', 'reason': 'Only 0.1% C code; minimal presence likely for platform-specific system calls'}, {'language': 'JavaScript', 'reason': 'No Node.js dependencies or package.json; no web framework dependencies detected'}
- **confidence**: 0.98

## Coding Patterns
- **error_handling**: Direct error checking with early returns and error propagation. Errors are checked immediately after operations and returned up the call stack. Uses standard Go error interface with custom error types from errdefs package.
- **testing_patterns**: Uses Go's standard testing package with table-driven tests and helper functions. Tests are organized in _test.go files alongside implementation. Integration tests use testing utilities and assertion libraries like gotest.tools.
- **async_patterns**: Uses goroutines and synchronization primitives for concurrency. Employs sync.Mutex and sync.Cond for thread-safe access to shared resources. Publisher-subscriber pattern used for event distribution.
- **file_organization**: Organized by functional domains with package-based structure. Related functionality grouped in packages (daemon, client, integration, pkg). Platform-specific code separated with _windows, _linux suffixes. Tests colocated with implementation in _test.go files.
- **code_style**: Follows Go conventions with CamelCase for exported identifiers and lowercase for unexported. Functional options pattern used for configuration. Interfaces defined for abstraction. Comments provided for exported types and functions.

## Key Utilities
*(No key utilities identified yet. These will populate as you build stories.)*

---

# Evolution

*(No stories completed yet.)*
