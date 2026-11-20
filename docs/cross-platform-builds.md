# Cross-Platform Builds (`docs/cross-platform-builds.md`)

Go AI Gateway uses Go's native cross-compilation (`GOOS`/`GOARCH`) and TinyGo WebAssembly to produce standalone binaries for **Windows**, **Linux**, **ARM64**, **Android**, **iOS**, and **Web (Wasm)** inside the `builds/` directory.

```mermaid
flowchart TD
  Src["Go Source (.)"] --> Sh["scripts/build-all.sh"]
  Sh --> Win["builds/windows-amd64/"]
  Sh --> Lin["builds/linux-amd64/"]
  Sh --> Arm["builds/linux-arm64/ & darwin-arm64/"]
  Sh --> And["builds/android-arm64/"]
  Sh --> IOS["builds/ios-arm64/ (macOS Xcode)"]
  Sh --> Web["builds/web/ (TinyGo Wasm)"]
  style Src fill:#dbeafe,stroke:#2563eb,color:#1e3a8a
  style Sh fill:#dcfce7,stroke:#16a34a,color:#14532d
  style Win fill:#e0f2fe,stroke:#0284c7,color:#0c4a6e
  style Lin fill:#fef9c3,stroke:#ca8a04,color:#713f12
  style Arm fill:#f3e8ff,stroke:#9333ea,color:#581c87
  style And fill:#dcfce7,stroke:#16a34a,color:#14532d
  style IOS fill:#ffedd5,stroke:#ea580c,color:#7c2d12
  style Web fill:#fee2e2,stroke:#dc2626,color:#7f1d1d
```

## Build Matrix

| Target | Output Path | Toolchain | Notes |
| --- | --- | --- | --- |
| **Windows (x64)** | `builds/windows-amd64/go-ai-gateway.exe` | `CGO_ENABLED=0 GOOS=windows GOARCH=amd64` | Standalone `.exe` |
| **Linux (x64)** | `builds/linux-amd64/go-ai-gateway` | `CGO_ENABLED=0 GOOS=linux GOARCH=amd64` | Static binary |
| **ARM64 (Linux)** | `builds/linux-arm64/go-ai-gateway` | `CGO_ENABLED=0 GOOS=linux GOARCH=arm64` | Raspberry Pi, AWS Graviton |
| **ARM64 (macOS)** | `builds/darwin-arm64/go-ai-gateway` | `CGO_ENABLED=0 GOOS=darwin GOARCH=arm64` | Apple Silicon |
| **Android (ARM64)** | `builds/android-arm64/go-ai-gateway` | `CGO_ENABLED=0 GOOS=android GOARCH=arm64` | Termux / embedded Android |
| **iOS (ARM64)** | `builds/ios-arm64/go-ai-gateway` | `CGO_ENABLED=1 GOOS=ios GOARCH=arm64` | Requires macOS + Xcode SDK |
| **Web (Wasm)** | `builds/web/app.wasm` | `tinygo build -target wasm -no-debug .` | Cloudflare Workers / Pages |

## Building & Running Dynamically

```mermaid
flowchart LR
  Run["scripts/run-gateway.sh"] --> Det["Detect OS / Arch"]
  Det --> Bin["Select builds/<target>"]
  Det -- "--web" --> Wasm["Wrangler + builds/web/app.wasm"]
  Bin --> Exec["exec go-ai-gateway"]
  style Run fill:#dbeafe,stroke:#2563eb,color:#1e3a8a
  style Det fill:#fef9c3,stroke:#ca8a04,color:#713f12
  style Bin fill:#dcfce7,stroke:#16a34a,color:#14532d
  style Wasm fill:#f3e8ff,stroke:#9333ea,color:#581c87
  style Exec fill:#e0f2fe,stroke:#0284c7,color:#0c4a6e
```

```bash
# 1. Build all targets into builds/
./scripts/build-all.sh

# 2. Dynamically detect OS/architecture and run the matching binary
./scripts/run-gateway.sh

# 3. Launch the WebAssembly build locally via Wrangler
./scripts/run-gateway.sh --web
```

