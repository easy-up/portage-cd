# Pipeline Debugging Guide

This document captures the history and debugging process for critical pipeline issues encountered during development.

## Semgrep OCaml Build Failures (September 2025)

### Problem Summary

Docker builds began failing across all branches (main, belay_main, v0.0.6) with OCaml dependency conflicts during semgrep-core compilation. This issue prevented new releases and container image builds.

### Root Cause Analysis

**Primary Issue**: OCaml ecosystem drift and opam repository evolution
- Original builds used OCaml 5.2.1 with semgrep v1.109.0
- OCaml 5.x introduced breaking changes affecting package compatibility
- The opam package repository evolved since January 2025, breaking historical dependency resolution

**Specific Error Patterns**:
1. **LSP Dependency Conflict**: `lsp = 1.20.0 no matching version`
2. **Cohttp Version Conflict**: `cohttp = 6.0.0` vs `cohttp = 2.5.8` 
3. **Compilation Errors**: Deprecated API usage in semgrep source with newer OCaml packages

### Investigation Process

#### Step 1: Version Analysis
Compared failing branches with successful commit `d44e522` (January 2025):
- **Working**: OCaml 4.14.0, semgrep v1.104.0, Alpine 3.19
- **Failing**: OCaml 5.2.1, semgrep v1.109.0, Alpine 3.20

#### Step 2: Systematic Testing
Tested multiple version combinations:
```dockerfile
# Attempted configurations:
OCaml 5.2.1 + semgrep v1.109.0 + Alpine 3.20 → LSP conflict
OCaml 4.14.0 + semgrep v1.109.0 + Alpine 3.19 → cohttp conflict  
OCaml 4.14.0 + semgrep v1.104.0 + Alpine 3.19 → cohttp conflict
OCaml 4.14.0 + semgrep v1.88.0 + Alpine 3.19 → compilation errors
OCaml 4.14.0 + semgrep v1.45.0 + Alpine 3.19 → package conflicts
```

#### Step 3: Official Semgrep Analysis
Compared our approach with official semgrep Dockerfile:
- **Official**: Alpine 3.21, OCaml 5.3.0, latest semgrep, includes Python runtime
- **Ours**: Alpine 3.19, OCaml 4.14.0, older semgrep, semgrep-core only

### Design Decision: Why Semgrep-Core Only

**Architectural Choice**: Build only semgrep-core (OCaml binary) without Python CLI wrapper

**Benefits**:
- **Container Size**: Avoids Python runtime (~50-100MB savings)
- **Security**: Reduced attack surface, fewer dependencies
- **Performance**: Direct OCaml execution without Python overhead
- **Simplicity**: Single-purpose binary for pattern matching

**Trade-offs**:
- **Rule Management**: No automatic rule downloading/updating
- **CLI Features**: Missing Python-based features (rule packaging, output formatting)
- **Compatibility**: Limited to semgrep-core API vs full semgrep CLI

### Current Status (v0.0.8)

**Working Components**:
- ✅ Go binary compilation (portage-cd core)
- ✅ Gatecheck integration from belay_main branch
- ✅ Grype, Syft, Gitleaks, ClamAV builds
- ✅ Container base image and runtime configuration

**Broken Components**:
- ❌ Semgrep-core compilation due to OCaml ecosystem drift
- ❌ Code scanning pipeline (semgrep-dependent features)

**Impact**: Primary portage-cd functionality and gatecheck workflow remain fully operational. Only static analysis features are affected.

### Potential Solutions

#### Option 1: Repository Pinning
Pin opam repository to historical state when dependencies worked:
```dockerfile
RUN opam repository add --set-default historical-opam <historical-snapshot-url>
```

#### Option 2: Pre-built Binaries  
Use official semgrep-core binaries instead of compilation:
```dockerfile
COPY --from=semgrep/semgrep:latest /usr/local/bin/semgrep-core /usr/local/bin/osemgrep
```

#### Option 3: Dependency Vendoring
Vendor specific OCaml package versions that worked in January 2025.

#### Option 4: Alternative Static Analysis
Replace semgrep with other static analysis tools:
- **CodeQL**: GitHub's static analysis engine
- **Bandit**: Python security linter  
- **ESLint**: JavaScript/TypeScript analysis
- **Custom Rules**: Pattern matching with grep/ripgrep

#### Option 5: Official Image Parsing
Extract semgrep-core from official container without Python runtime:
```dockerfile
FROM semgrep/semgrep:latest AS semgrep-extract
FROM alpine:3.20
COPY --from=semgrep-extract /usr/local/bin/semgrep-core /usr/local/bin/osemgrep
```

### Lessons Learned

1. **Dependency Pinning**: Pin critical build dependencies to avoid ecosystem drift
2. **Build Validation**: Implement regular build testing to catch ecosystem changes early  
3. **Fallback Strategies**: Design pipeline to gracefully handle tool failures
4. **Documentation**: Capture build reasoning and troubleshooting steps
5. **Alternative Tools**: Maintain options for replacing problematic dependencies

### Configuration History

#### Working Configuration (January 2025 - Commit d44e522)
```dockerfile
ARG ALPINE_VERSION=3.20
FROM alpine:3.19 AS build-semgrep-core  # Note: Hard-coded 3.19
ARG OCAML_VERSION=4.14.0
ARG SEMGREP_VERSION=v1.104.0
RUN make install-deps-ALPINE-for-semgrep-core
RUN apk add --no-cache zstd libpsl-dev
RUN make install-deps-for-semgrep-core
```

#### Current Broken State (September 2025)
```dockerfile
ARG ALPINE_VERSION=3.20  
FROM alpine:3.19 AS build-semgrep-core
ARG OCAML_VERSION=4.14.0  # Reverted from 5.2.1
ARG SEMGREP_VERSION=v1.104.0  # Reverted from v1.109.0
# Same build sequence but opam repository has evolved
```

### Monitoring and Prevention

**Build Health Checks**:
- Weekly automated builds to detect ecosystem drift
- Dependency version tracking in CI/CD
- Container security scanning for dependency vulnerabilities

**Documentation Requirements**:
- Document all external dependencies with version constraints
- Maintain troubleshooting guides for common build failures
- Track upstream project changes affecting our integrations

---

*This document should be updated whenever significant pipeline debugging occurs or when the semgrep build issue is resolved.*
