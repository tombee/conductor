# Installation

This guide covers all available methods to install Conductor on your system.

## System Requirements

- **Operating System**: macOS, Linux, or Windows
- **Go** (if installing from source): 1.21 or later
- **Optional**: Homebrew (macOS/Linux)

## Installation Methods

Choose the method that best fits your workflow:

### Homebrew (Recommended for macOS/Linux)

The easiest way to install Conductor on macOS or Linux is via Homebrew:

```bash
brew install conductor
```

Verify the installation:

```bash
conductor --version
```

**Updating with Homebrew:**

```bash
brew update
brew upgrade conductor
```

**Uninstalling:**

```bash
brew uninstall conductor
```

### Go Install

If you have Go installed, you can install Conductor directly:

```bash
go install github.com/tombee/conductor/cmd/conductor@latest
```

:::note[GOPATH Configuration]
Ensure `$GOPATH/bin` is in your `PATH`:

```bash
export PATH=$PATH:$(go env GOPATH)/bin
```

Add this to your shell profile (`~/.zshrc`, `~/.bashrc`) to make it permanent.
:::


Verify the installation:

```bash
conductor --version
```

**Installing a Specific Version:**

```bash
go install github.com/tombee/conductor/cmd/conductor@v1.2.3
```

**Updating:**

Re-run the install command to get the latest version:

```bash
go install github.com/tombee/conductor/cmd/conductor@latest
```

### Binary Download

Download pre-built binaries from the [GitHub Releases page](https://github.com/tombee/conductor/releases):

1. Navigate to the [latest release](https://github.com/tombee/conductor/releases/latest)
2. Download the appropriate archive for your platform:
   - macOS (Intel): `conductor_darwin_amd64.tar.gz`
   - macOS (Apple Silicon): `conductor_darwin_arm64.tar.gz`
   - Linux (x86_64): `conductor_linux_amd64.tar.gz`
   - Linux (ARM64): `conductor_linux_arm64.tar.gz`
   - Windows (x64): `conductor_windows_amd64.zip`

3. Extract the archive:

   === "macOS/Linux"

       ```bash
       tar -xzf conductor_*.tar.gz
       ```

   === "Windows"

       Use Windows Explorer to extract the ZIP file, or use PowerShell:

       ```powershell
       Expand-Archive conductor_windows_amd64.zip
       ```

4. Move the binary to a location in your PATH:

   === "macOS/Linux"

       ```bash
       sudo mv conductor /usr/local/bin/
       sudo chmod +x /usr/local/bin/conductor
       ```

   === "Windows"

       Move `conductor.exe` to a directory in your PATH, such as `C:\Program Files\Conductor\`

5. Verify the installation:

   ```bash
   conductor --version
   ```

### From Source

Build Conductor from source for the latest development version or custom builds:

1. Clone the repository:

   ```bash
   git clone https://github.com/tombee/conductor.git
   cd conductor
   ```

2. Build and install:

   ```bash
   go install ./cmd/conductor
   ```

   Or build without installing:

   ```bash
   go build -o conductor ./cmd/conductor
   ```

3. (Optional) Move the binary to your PATH:

   ```bash
   sudo mv conductor /usr/local/bin/
   ```

4. Verify the installation:

   ```bash
   conductor --version
   ```

**Building for Other Platforms:**

Cross-compile for different platforms:

```bash
# For Linux AMD64
GOOS=linux GOARCH=amd64 go build -o conductor-linux ./cmd/conductor

# For Windows AMD64
GOOS=windows GOARCH=amd64 go build -o conductor.exe ./cmd/conductor

# For macOS ARM64 (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o conductor-darwin-arm64 ./cmd/conductor
```

## Docker

Run Conductor in a container without local installation:

```bash
docker run --rm -v $(pwd):/workspace -e ANTHROPIC_API_KEY ghcr.io/tombee/conductor:latest run /workspace/workflow.yaml
```

:::tip[Docker Compose]
For daemon mode, use Docker Compose:

```yaml
version: '3.8'
services:
  conductor:
    image: ghcr.io/tombee/conductor:latest
    command: daemon
    environment:
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
    volumes:
      - ./workflows:/workflows
      - conductor-data:/data
    ports:
      - "8080:8080"
volumes:
  conductor-data:
```
:::


## Verification

After installation, verify Conductor is working correctly:

```bash
# Check version
conductor --version

# View available commands
conductor --help

# List available tools
conductor tools list
```

Expected output:

```
conductor version 1.0.0
```

## Next Steps

Now that Conductor is installed:

1. **Configure API Keys**: [Configuration Reference](../../reference/configuration.md)
2. **Run Your First Workflow**: [First Workflow Tutorial](first-workflow.md)
3. **Quick Start**: [Quick Start Guide](../../quick-start.md)

## Troubleshooting

### Command Not Found

**Problem**: Shell can't find the `conductor` command.

**Solution**: Add the installation directory to your PATH:

=== "Homebrew"

    Homebrew should do this automatically. If not:

    ```bash
    echo 'export PATH="/usr/local/bin:$PATH"' >> ~/.zshrc
    source ~/.zshrc
    ```

=== "Go Install"

    Ensure `$GOPATH/bin` is in your PATH:

    ```bash
    echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.zshrc
    source ~/.zshrc
    ```

=== "Binary Download"

    Add the directory containing `conductor` to your PATH:

    ```bash
    echo 'export PATH=$PATH:/usr/local/bin' >> ~/.zshrc
    source ~/.zshrc
    ```

### Permission Denied

**Problem**: Cannot execute the conductor binary.

**Solution**: Make the binary executable:

```bash
chmod +x /usr/local/bin/conductor
```

### Go Version Too Old

**Problem**: `go install` fails with version error.

**Solution**: Update Go to version 1.21 or later:

```bash
# macOS (Homebrew)
brew upgrade go

# Linux (download from golang.org)
wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
```

### Docker Image Not Found

**Problem**: Docker can't pull the conductor image.

**Solution**: Check the latest image tag on [GitHub Container Registry](https://github.com/tombee/conductor/pkgs/container/conductor):

```bash
docker pull ghcr.io/tombee/conductor:latest
```

## Alternative Installation Locations

If you prefer to install Conductor in a different location:

### User-Local Installation (No sudo required)

```bash
# Create a local bin directory
mkdir -p ~/.local/bin

# Move conductor there
mv conductor ~/.local/bin/

# Add to PATH (if not already present)
echo 'export PATH=$PATH:$HOME/.local/bin' >> ~/.zshrc
source ~/.zshrc
```

### Project-Local Installation

For project-specific installations:

```bash
# In your project directory
mkdir -p .bin
mv conductor .bin/

# Use with ./bin/conductor or add to PATH temporarily
export PATH=$PATH:$(pwd)/.bin
```

## Getting Help

- **Full Documentation**: [Documentation Home](../../index.md)
- **GitHub Issues**: [Report installation problems](https://github.com/tombee/conductor/issues)
- **Discussions**: [Ask questions](https://github.com/tombee/conductor/discussions)
