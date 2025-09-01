#!/bin/bash

# Installation automatique du serveur MCP dans Claude Code

MCP_NAME="mcp-xlsm"
MCP_PATH="$(pwd)/mcp-xlsm-server"
CONFIG_PATH="$(pwd)/config.yaml"

echo "Installation du serveur MCP XLSM dans Claude Code..."

# Vérifier que le binaire existe
if [ ! -f "$MCP_PATH" ]; then
    echo "Erreur: Binaire non trouvé à $MCP_PATH"
    echo "Exécutez d'abord: make build"
    exit 1
fi

# Vérifier que le config existe
if [ ! -f "$CONFIG_PATH" ]; then
    echo "Erreur: Configuration non trouvée à $CONFIG_PATH"
    exit 1
fi

# Installer le serveur MCP
claude mcp add $MCP_NAME "$MCP_PATH" --scope user -- --stdio --config "$CONFIG_PATH"

if [ $? -eq 0 ]; then
    echo "✓ Serveur MCP installé avec succès"
    echo ""
    echo "Pour tester dans Claude Code:"
    echo "1. Ouvrez Claude Code"
    echo "2. Le serveur MCP devrait être disponible"
    echo "3. Testez avec: 'Analyse le fichier Excel /Volumes/SSD/MCP/COMBINE INTERACTIF 02 2025.xlsm'"
else
    echo "✗ Échec de l'installation"
    exit 1
fi
