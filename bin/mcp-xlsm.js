#!/usr/bin/env node

const { spawn } = require('child_process');
const path = require('path');
const os = require('os');
const fs = require('fs');

// Get the platform-specific binary name
function getBinaryName() {
  const platform = os.platform();
  const arch = os.arch();
  
  let binaryName = 'mcp-xlsm-server';
  
  if (platform === 'win32') {
    binaryName += '.exe';
  }
  
  return binaryName;
}

// Get the binary path
function getBinaryPath() {
  const binaryName = getBinaryName();
  const binDir = path.dirname(__filename);
  const binaryPath = path.join(binDir, binaryName);
  
  if (!fs.existsSync(binaryPath)) {
    console.error(`Binary not found at: ${binaryPath}`);
    console.error('Please run: npm run postinstall');
    process.exit(1);
  }
  
  return binaryPath;
}

// Get config path
function getConfigPath() {
  const packageRoot = path.resolve(__dirname, '..');
  const configPath = path.join(packageRoot, 'config.yaml');
  
  if (!fs.existsSync(configPath)) {
    console.error(`Config file not found at: ${configPath}`);
    process.exit(1);
  }
  
  return configPath;
}

// Main execution
function main() {
  const binaryPath = getBinaryPath();
  const configPath = getConfigPath();
  
  // Parse arguments
  const args = process.argv.slice(2);
  
  // Add default config if not specified
  if (!args.includes('--config')) {
    args.push('--config', configPath);
  }
  
  // Check if running in stdio mode for MCP
  const isStdio = args.includes('--stdio');
  
  if (isStdio) {
    console.error('Starting MCP XLSM Server in stdio mode...');
  } else {
    console.log('Starting MCP XLSM Server in HTTP mode...');
    console.log(`Server will be available at http://localhost:3001`);
  }
  
  // Spawn the binary
  const child = spawn(binaryPath, args, {
    stdio: isStdio ? 'inherit' : 'inherit',
    env: process.env
  });
  
  child.on('error', (err) => {
    console.error('Failed to start server:', err);
    process.exit(1);
  });
  
  child.on('exit', (code, signal) => {
    if (signal) {
      console.log(`Server terminated by signal: ${signal}`);
    } else if (code !== 0) {
      console.error(`Server exited with code: ${code}`);
    }
    process.exit(code || 0);
  });
  
  // Handle termination signals
  process.on('SIGINT', () => {
    child.kill('SIGINT');
  });
  
  process.on('SIGTERM', () => {
    child.kill('SIGTERM');
  });
}

// Run
main();