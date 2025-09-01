#!/usr/bin/env node

const { execSync } = require('child_process');
const path = require('path');
const fs = require('fs');
const os = require('os');

const BINARY_NAME = 'mcp-xlsm-server';
const PACKAGE_ROOT = path.resolve(__dirname, '..');
const BIN_DIR = path.join(PACKAGE_ROOT, 'bin');

function log(message) {
  console.log(`[postinstall] ${message}`);
}

function error(message) {
  console.error(`[postinstall] ERROR: ${message}`);
}

// Check if Go is installed
function checkGo() {
  try {
    const version = execSync('go version', { encoding: 'utf8' });
    log(`Found Go: ${version.trim()}`);
    return true;
  } catch (e) {
    error('Go is not installed. Please install Go 1.21+ to build the server.');
    error('Visit: https://golang.org/dl/');
    return false;
  }
}

// Build the Go binary
function buildBinary() {
  log('Building MCP XLSM Server binary...');
  
  const platform = os.platform();
  let binaryPath = path.join(BIN_DIR, BINARY_NAME);
  
  if (platform === 'win32') {
    binaryPath += '.exe';
  }
  
  try {
    // Change to package root directory
    process.chdir(PACKAGE_ROOT);
    
    // Build the binary
    const buildCmd = `go build -ldflags "-X main.version=2.0.0" -o "${binaryPath}" ./cmd`;
    log(`Running: ${buildCmd}`);
    
    execSync(buildCmd, { 
      stdio: 'inherit',
      env: {
        ...process.env,
        CGO_ENABLED: '0',
        GOOS: platform === 'win32' ? 'windows' : platform,
        GOARCH: os.arch() === 'x64' ? 'amd64' : os.arch()
      }
    });
    
    // Make binary executable on Unix systems
    if (platform !== 'win32') {
      fs.chmodSync(binaryPath, '755');
    }
    
    log(`✓ Binary built successfully: ${binaryPath}`);
    return true;
  } catch (e) {
    error(`Failed to build binary: ${e.message}`);
    return false;
  }
}

// Download pre-built binary (fallback)
async function downloadBinary() {
  log('Attempting to download pre-built binary...');
  
  const platform = os.platform();
  const arch = os.arch();
  
  // Map Node.js platform/arch to Go equivalents
  const platformMap = {
    'darwin': 'darwin',
    'linux': 'linux',
    'win32': 'windows'
  };
  
  const archMap = {
    'x64': 'amd64',
    'arm64': 'arm64'
  };
  
  const goPlatform = platformMap[platform];
  const goArch = archMap[arch];
  
  if (!goPlatform || !goArch) {
    error(`Unsupported platform: ${platform}/${arch}`);
    return false;
  }
  
  const binaryName = platform === 'win32' ? `${BINARY_NAME}.exe` : BINARY_NAME;
  const downloadUrl = `https://github.com/yourusername/mcp-xlsm-server/releases/latest/download/${BINARY_NAME}-${goPlatform}-${goArch}${platform === 'win32' ? '.exe' : ''}`;
  
  log(`Download URL: ${downloadUrl}`);
  
  try {
    const https = require('https');
    const binaryPath = path.join(BIN_DIR, binaryName);
    
    await new Promise((resolve, reject) => {
      const file = fs.createWriteStream(binaryPath);
      
      https.get(downloadUrl, (response) => {
        if (response.statusCode === 302 || response.statusCode === 301) {
          // Follow redirect
          https.get(response.headers.location, (redirectResponse) => {
            redirectResponse.pipe(file);
            file.on('finish', () => {
              file.close();
              fs.chmodSync(binaryPath, '755');
              resolve();
            });
          }).on('error', reject);
        } else if (response.statusCode === 200) {
          response.pipe(file);
          file.on('finish', () => {
            file.close();
            fs.chmodSync(binaryPath, '755');
            resolve();
          });
        } else {
          reject(new Error(`HTTP ${response.statusCode}`));
        }
      }).on('error', reject);
    });
    
    log(`✓ Binary downloaded successfully`);
    return true;
  } catch (e) {
    error(`Failed to download binary: ${e.message}`);
    return false;
  }
}

// Main installation flow
async function install() {
  log('Starting MCP XLSM Server installation...');
  
  // Ensure bin directory exists
  if (!fs.existsSync(BIN_DIR)) {
    fs.mkdirSync(BIN_DIR, { recursive: true });
  }
  
  // Check if binary already exists
  const platform = os.platform();
  const binaryName = platform === 'win32' ? `${BINARY_NAME}.exe` : BINARY_NAME;
  const binaryPath = path.join(BIN_DIR, binaryName);
  
  if (fs.existsSync(binaryPath)) {
    log('Binary already exists, skipping build.');
    return;
  }
  
  // Try to build from source
  if (checkGo()) {
    if (buildBinary()) {
      log('✓ Installation complete!');
      log('Run "npx mcp-xlsm" to start the server');
      return;
    }
  }
  
  // Fallback to downloading pre-built binary
  log('Attempting fallback installation...');
  if (await downloadBinary()) {
    log('✓ Installation complete!');
    log('Run "npx mcp-xlsm" to start the server');
  } else {
    error('Installation failed. Please build manually:');
    error('  cd ' + PACKAGE_ROOT);
    error('  go build -o bin/' + binaryName + ' ./cmd');
    process.exit(1);
  }
}

// Run installation
install().catch(err => {
  error(`Installation failed: ${err.message}`);
  process.exit(1);
});