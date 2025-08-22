# Helm Diff Plugin

Helm Diff is a Go-based Helm plugin that provides diff functionality for comparing Helm charts and releases. It shows what changes would occur during helm upgrade, rollback, or between different releases/revisions.

Always reference these instructions first and fallback to search or bash commands only when you encounter unexpected information that does not match the info here.

## Working Effectively

**Prerequisites:**
- Go >= 1.21 (currently uses Go 1.24.5)
- Helm v3 (tested with v3.17.4 and v3.18.6)
- Make sure `/home/runner/go/bin` is in your PATH for staticcheck: `export PATH=$PATH:/home/runner/go/bin`

**Bootstrap and Build Process:**
- ALWAYS run: `make bootstrap` first - downloads dependencies and installs staticcheck. Takes <1 second (if already done) or ~50 seconds (first time).
- Build the plugin: `make build` - includes linting and compiles the binary. Takes ~9 seconds after bootstrap.
- NEVER CANCEL builds. Set timeout to 3+ minutes for bootstrap, 2+ minutes for build operations.

**Testing:**
- Run unit tests: `make test` - includes coverage analysis. Takes ~12 seconds. NEVER CANCEL - set timeout to 3+ minutes.
- Tests include comprehensive coverage (38.7% overall) and use a fake helm binary for isolation.
- Test coverage is generated in `cover.out` with detailed function-level coverage reports.

**Linting and Code Quality:**
- Local linting: `make lint` - runs gofmt, go vet, and staticcheck verification. Takes ~2 seconds.
- Code formatting: `make format` - applies gofmt formatting automatically. Takes <1 second.
- Full golangci-lint runs only in CI via GitHub Actions, not available locally.
- ALWAYS run `make format` and `make lint` before committing changes.

**Plugin Installation:**
- Install as Helm plugin: `make install` or `make install/helm3` - builds and installs to Helm plugins directory. Takes ~3 seconds.
- The plugin installs via `install-binary.sh` script which handles cross-platform binary installation.

## Validation Scenarios

**ALWAYS test your changes with these scenarios:**

1. **Basic Plugin Functionality:**
   ```bash
   # Test the binary directly
   ./bin/diff version
   ./bin/diff --help
   ./bin/diff upgrade --help
   ```

2. **Real Chart Diffing:**
   ```bash
   # Create a test chart and diff it
   cd /tmp && helm create test-chart
   cd /path/to/helm-diff
   HELM_NAMESPACE=default HELM_BIN=helm ./bin/diff upgrade --install --dry-run test-release /tmp/test-chart
   ```

3. **Plugin Installation Verification:**
   ```bash
   # Test plugin installation
   export HELM_DATA_HOME=/tmp/helm-test
   make install
   /tmp/helm-test/plugins/helm-diff/bin/diff version
   ```

## Build Times and Timeouts

**CRITICAL: NEVER CANCEL long-running commands. Use these timeout values:**

- `make bootstrap`: <1 second (if already done) or ~50 seconds (first time) (set timeout: 5+ minutes)
- `make build`: ~9 seconds after bootstrap (set timeout: 3+ minutes)
- `make test`: ~12 seconds (set timeout: 3+ minutes)
- `make lint`: ~2 seconds (set timeout: 1 minute)
- `make format`: <1 second (set timeout: 1 minute)
- `make install`: ~3 seconds (set timeout: 2 minutes)

## Common Tasks

**Repository Structure:**
- `main.go` - Entry point that delegates to cmd package
- `cmd/` - Command-line interface implementation (upgrade, release, revision, rollback, version)
- `diff/` - Core diffing logic and output formatting
- `manifest/` - Kubernetes manifest parsing and handling
- `scripts/` - Build and verification scripts (gofmt, govet, staticcheck)
- `testdata/`, `*/testdata/` - Test fixtures and mock data
- `plugin.yaml` - Helm plugin configuration
- `install-binary.sh` - Cross-platform installation script
- `Makefile` - Build system with all common targets

**Key Files to Check After Changes:**
- Always run tests after modifying `cmd/` or `diff/` packages
- Check `plugin.yaml` version if making release changes
- Verify `Makefile` targets if changing build process
- Review `install-binary.sh` if modifying installation process

**Environment Variables for Testing:**
- `HELM_NAMESPACE` - Kubernetes namespace for operations
- `HELM_BIN` - Path to helm binary (for direct testing)
- `HELM_DIFF_USE_UPGRADE_DRY_RUN` - Use helm upgrade --dry-run instead of template
- `HELM_DIFF_THREE_WAY_MERGE` - Enable three-way merge diffing
- `HELM_DIFF_NORMALIZE_MANIFESTS` - Normalize YAML before diffing
- `HELM_DIFF_OUTPUT_CONTEXT` - Configure output context lines

**CI/CD Information:**
- GitHub Actions runs on push/PR to master branch
- Tests run on Ubuntu, macOS, Windows with multiple Helm versions
- Integration tests use Kind (Kubernetes in Docker)
- Linting uses golangci-lint via GitHub Actions (not available locally)
- Cross-platform plugin installation is tested via Docker

**Direct Binary Usage (for development):**
```bash
# Build and test directly without Helm plugin installation
go build -o bin/diff -ldflags="-X github.com/databus23/helm-diff/v3/cmd.Version=dev"
HELM_NAMESPACE=default HELM_BIN=helm ./bin/diff upgrade --install --dry-run my-release ./chart-path
```

**Common Commands Reference:**
```bash
# Full development cycle
make bootstrap      # Install dependencies (once)
make build         # Build with linting  
make test          # Run all tests
make format        # Format code
make install       # Install as Helm plugin

# Validation
./bin/diff version                    # Check version
./bin/diff upgrade --help            # Check help
make test                           # Run test suite
```

## Important Notes

- The plugin supports Helm v3 only (v2 support was removed)
- Uses Go modules for dependency management
- Cross-platform support: Linux, macOS, Windows, FreeBSD
- Multiple output formats: diff, simple, template, dyff
- Supports both client-side and server-side dry-run modes
- Includes three-way merge capabilities for advanced diffing
- Plugin binary is named `diff` and installed as `helm diff` command