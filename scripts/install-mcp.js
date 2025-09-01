#!/usr/bin/env node

const { execSync, spawn } = require('child_process');
const path = require('path');
const fs = require('fs');
const os = require('os');

const MCP_NAME = 'mcp-xlsm';
const PACKAGE_ROOT = path.resolve(__dirname, '..');
const BIN_DIR = path.join(PACKAGE_ROOT, 'bin');

function log(message) {
  console.log(`[install-mcp] ${message}`);
}

function error(message) {
  console.error(`[install-mcp] ERROR: ${message}`);
}

function success(message) {
  console.log(`[install-mcp] ✓ ${message}`);
}

// Check if Claude CLI is installed
function checkClaudeCLI() {
  try {
    const version = execSync('claude --version', { encoding: 'utf8' });
    log(`Found Claude CLI: ${version.trim()}`);
    return true;
  } catch (e) {
    error('Claude CLI is not installed.');
    error('Please install it first:');
    error('  brew install claude (macOS)');
    error('  or visit: https://claude.ai/cli');
    return false;
  }
}

// Get binary path
function getBinaryPath() {
  const platform = os.platform();
  const binaryName = platform === 'win32' ? 'mcp-xlsm-server.exe' : 'mcp-xlsm-server';
  const binaryPath = path.join(BIN_DIR, binaryName);
  
  if (!fs.existsSync(binaryPath)) {
    error(`Binary not found at: ${binaryPath}`);
    error('Please run: npm run postinstall');
    return null;
  }
  
  return binaryPath;
}

// Get config path
function getConfigPath() {
  const configPath = path.join(PACKAGE_ROOT, 'config.yaml');
  
  if (!fs.existsSync(configPath)) {
    error(`Config file not found at: ${configPath}`);
    return null;
  }
  
  return configPath;
}

// Install MCP server in Claude Code
function installMCP() {
  const binaryPath = getBinaryPath();
  const configPath = getConfigPath();
  
  if (!binaryPath || !configPath) {
    return false;
  }
  
  log(`Installing MCP server: ${MCP_NAME}`);
  log(`Binary: ${binaryPath}`);
  log(`Config: ${configPath}`);
  
  try {
    // Build the command
    const args = [
      'mcp', 'add',
      MCP_NAME,
      binaryPath,
      '--scope', 'user',
      '--',
      '--stdio',
      '--config', configPath
    ];
    
    log(`Running: claude ${args.join(' ')}`);
    
    // Execute the command
    const result = execSync(`claude ${args.join(' ')}`, {
      encoding: 'utf8',
      stdio: 'pipe'
    });
    
    success('MCP server installed successfully!');
    
    if (result) {
      log('Output:', result);
    }
    
    return true;
  } catch (e) {
    if (e.message.includes('already exists')) {
      log('MCP server already installed. Updating...');
      
      try {
        // Remove existing
        execSync(`claude mcp remove ${MCP_NAME} --scope user`, {
          encoding: 'utf8',
          stdio: 'pipe'
        });
        
        // Reinstall
        return installMCP();
      } catch (removeError) {
        error(`Failed to update: ${removeError.message}`);
      }
    } else {
      error(`Failed to install: ${e.message}`);
    }
    return false;
  }
}

// Test the installation
function testInstallation() {
  log('Testing MCP server...');
  
  const binaryPath = getBinaryPath();
  const configPath = getConfigPath();
  
  if (!binaryPath || !configPath) {
    return false;
  }
  
  try {
    // Test with initialize command
    const testCommand = '{"jsonrpc":"2.0","method":"initialize","params":{"clientInfo":{"name":"test","version":"1.0.0"}},"id":1}';
    
    const result = execSync(`echo '${testCommand}' | "${binaryPath}" --stdio --config "${configPath}"`, {
      encoding: 'utf8',
      stdio: 'pipe'
    });
    
    if (result.includes('"result"')) {
      success('MCP server test passed!');
      return true;
    } else {
      error('MCP server test failed - unexpected response');
      return false;
    }
  } catch (e) {
    error(`Test failed: ${e.message}`);
    return false;
  }
}

// Print usage instructions
function printUsage() {
  console.log('\n' + '='.repeat(60));
  console.log('MCP XLSM Server installed successfully!');
  console.log('='.repeat(60));
  console.log('\nUsage in Claude Code:');
  console.log('  1. Open Claude Code');
  console.log('  2. The MCP server should be available automatically');
  console.log('  3. Try these commands:');
  console.log('     - "Analyze the Excel file /path/to/file.xlsm"');
  console.log('     - "Show me the data from sheet FROUDIS"');
  console.log('     - "Search for revenue across all sheets"');
  console.log('\nManagement commands:');
  console.log('  npm run install-mcp    # Reinstall/update MCP server');
  console.log('  npm run uninstall-mcp  # Remove MCP server');
  console.log('  npm start              # Run in HTTP mode (testing)');
  console.log('  npm run start:stdio    # Run in stdio mode (testing)');
  console.log('\nTroubleshooting:');
  console.log('  - Check logs at: ~/.claude/logs/');
  console.log('  - Test server: npm run test');
  console.log('  - Rebuild: npm run build');
  console.log('\n' + '='.repeat(60));
}

// Main installation flow
async function main() {
  log('Starting MCP XLSM Server installation for Claude Code...\n');
  
  // Check prerequisites
  if (!checkClaudeCLI()) {
    process.exit(1);
  }
  
  // Install MCP
  if (!installMCP()) {
    error('Installation failed');
    process.exit(1);
  }
  
  // Test installation
  if (!testInstallation()) {
    error('Warning: Test failed but installation may still work');
  }
  
  // Print usage
  printUsage();
}

// Handle uninstall option
if (process.argv.includes('--uninstall')) {
  log('Uninstalling MCP server...');
  try {
    execSync(`claude mcp remove ${MCP_NAME} --scope user`, {
      encoding: 'utf8',
      stdio: 'inherit'
    });
    success('MCP server uninstalled');
  } catch (e) {
    error(`Failed to uninstall: ${e.message}`);
    process.exit(1);
  }
} else {
  // Run installation
  main().catch(err => {
    error(`Installation failed: ${err.message}`);
    process.exit(1);
  });
}