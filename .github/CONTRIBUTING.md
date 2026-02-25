# 🤝 Contributing to VNX

Thank you for your interest in contributing to VNX! We welcome contributions from the community. This document provides guidelines and information to help you get started.

## 📋 Table of Contents

- [Getting Started](#getting-started)
- [Project Structure](#project-structure)
- [Development Setup](#development-setup)
- [Running the Application](#running-the-application)
- [Building for Production](#building-for-production)
- [Development Guidelines](#development-guidelines)
- [Submitting Changes](#submitting-changes)
- [Code Style](#code-style)
- [Testing](#testing)

## 🚀 Getting Started

### Prerequisites

Before you begin, ensure you have the following installed:

- **Go**: Version 1.21 or higher ([Download here](https://golang.org/dl/))
- **Git**: For version control
- **Web Browser**: Modern browser for testing

### Quick Setup

```bash
# Clone the repository
git clone https://github.com/thuongtruong109/vnx.git
cd vnx

# Install Go dependencies
go mod download

# Start the development server
go run cmd/main.go
```

Then open [http://localhost:8080](http://localhost:8080) in your browser.

## 📁 Project Structure

```
vnx/
├── cmd/
│   └── main.go              # Application entry point
├── internal/
│   ├── handlers/            # HTTP request handlers
│   │   ├── handlers.go      # Main handlers
│   │   ├── resolve.go       # Address resolution logic
│   ├── loader/              # Data loading utilities
│   │   └── loader.go        # JSON data loader
│   └── models/              # Data structures
│       └── models.go        # Address models
├── data/
│   ├── v1/                  # Old administrative data (pre-2025)
│   │   └── *.json           # Province/district/ward data
│   └── v2/                  # New administrative data (post-2025)
│       └── *.json           # Updated administrative divisions
├── static/
│   └── index.html           # Frontend application
├── scripts/                 # Data processing scripts
│   ├── enrich_v1.py         # Data enrichment scripts
│   ├── generate_map.py      # Mapping generation
│   ├── pretty_output.py     # Output formatting
│   └── update_data.py       # Data update utilities
└── README.md                # Project documentation
```

## 🛠️ Development Setup

### 1. Fork and Clone

```bash
# Fork the repository on GitHub, then clone your fork
git clone https://github.com/YOUR_USERNAME/vnx.git
cd vnx

# Add upstream remote
git remote add upstream https://github.com/thuongtruong109/vnx.git
```

### 2. Set up Development Environment

```bash
# Install Go dependencies
go mod download

# Optional: Set up a virtual environment for Python scripts
python -m venv .venv
source .venv/bin/activate  # On Windows: .venv\Scripts\activate
pip install -r requirements.txt  # If requirements.txt exists
```

### 3. Verify Setup

```bash
# Check Go version
go version

# Run tests to ensure everything works
go test ./...

# Start the server
go run cmd/main.go
```

## ▶️ Running the Application

### Development Mode

```bash
# Run with live reload (if using tools like air)
go run cmd/main.go

# Or use air for hot reloading
go install github.com/cosmtrek/air@latest
air
```

### Production Mode

```bash
# Build the application
go build -o vnx cmd/main.go

# Run the binary
./vnx
```

### Testing the API

The application provides REST endpoints for address conversion:

```bash
# Test province conversion
curl "http://localhost:8080/api/resolve?province=hanoi"

# Test full address conversion
curl "http://localhost:8080/api/resolve?province=hanoi&district=hoankiem&ward=hangdao"
```

## 🏗️ Building for Production

### Standard Build

```bash
# Build for current platform
go build -o vnx cmd/main.go

# Build with optimizations
go build -ldflags="-s -w" -o vnx cmd/main.go
```

### Cross-Platform Build

```bash
# Build for Linux
GOOS=linux GOARCH=amd64 go build -o vnx-linux cmd/main.go

# Build for Windows
GOOS=windows GOARCH=amd64 go build -o vnx.exe cmd/main.go

# Build for macOS
GOOS=darwin GOARCH=amd64 go build -o vnx-mac cmd/main.go
```

## 📝 Development Guidelines

### Code Quality

- Follow Go best practices and idioms
- Use meaningful variable and function names
- Add comments for complex logic
- Keep functions small and focused
- Handle errors appropriately

### Commit Messages

Use clear, descriptive commit messages:

```bash
# Good examples
git commit -m "feat: add support for ward-level address conversion"
git commit -m "fix: handle edge case in province name matching"
git commit -m "docs: update API documentation"

# Bad examples
git commit -m "fix bug"
git commit -m "update"
```

### Branch Naming

```bash
# Feature branches
git checkout -b feature/add-address-validation

# Bug fix branches
git checkout -b fix/province-mapping-issue

# Documentation branches
git checkout -b docs/update-contributing-guide
```

## 🔄 Submitting Changes

### 1. Create a Branch

```bash
# Create and switch to a new branch
git checkout -b feature/your-feature-name

# Or for bug fixes
git checkout -b fix/issue-description
```

### 2. Make Changes

- Write clean, tested code
- Update documentation if needed
- Add tests for new features
- Ensure all tests pass

### 3. Test Your Changes

```bash
# Run Go tests
go test ./...

# Run with race detector
go test -race ./...

# Build and test manually
go build -o vnx cmd/main.go
./vnx
```

### 4. Submit a Pull Request

```bash
# Push your branch
git push origin feature/your-feature-name

# Create a Pull Request on GitHub
# - Use a clear title
# - Provide detailed description
# - Reference any related issues
# - Request review from maintainers
```

## 🎨 Code Style

### Go Code

- Use `gofmt` to format your code
- Follow the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use `go vet` to check for common mistakes
- Keep line length reasonable (< 100 characters)

### HTML/CSS/JavaScript

- Use consistent indentation (2 spaces)
- Follow semantic HTML practices
- Use CSS custom properties for theming
- Keep JavaScript vanilla when possible

## 🧪 Testing

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific test
go test -run TestAddressResolution ./...
```

### Writing Tests

```go
func TestAddressResolution(t *testing.T) {
    // Test cases
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"valid province", "hanoi", "ha noi"},
        // Add more test cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := resolveAddress(tt.input)
            if result != tt.expected {
                t.Errorf("resolveAddress(%q) = %q, want %q", tt.input, result, tt.expected)
            }
        })
    }
}
```

## 📞 Getting Help

- **Issues**: [GitHub Issues](https://github.com/thuongtruong109/vnx/issues)
- **Discussions**: [GitHub Discussions](https://github.com/thuongtruong109/vnx/discussions)
- **Documentation**: Check the [README.md](README.md) first

## 🙏 Recognition

Contributors will be recognized in the project documentation and release notes. Thank you for helping make VNX better!

---

**Happy contributing! 🎉**</content>
<parameter name="filePath">c:\Users\admin\Downloads\Projects\vnx\CONTRIBUTING.md
