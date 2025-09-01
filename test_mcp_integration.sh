#!/bin/bash

# Test d'intégration MCP avec Claude Code
# ========================================

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

TEST_FILE="/Volumes/SSD/MCP/COMBINE INTERACTIF 02 2025.xlsm"
MCP_DIR=$(pwd)

echo -e "${BLUE}=== TEST D'INTÉGRATION MCP AVEC CLAUDE CODE ===${NC}\n"

# Étape 1: Vérifier l'installation de Claude CLI
echo -e "${YELLOW}ÉTAPE 1: Vérification de Claude CLI${NC}"
echo "------------------------------------"
if command -v claude &> /dev/null; then
    echo -e "${GREEN}✓ Claude CLI installé${NC}"
    claude --version
else
    echo -e "${RED}✗ Claude CLI non trouvé${NC}"
    echo "  Installez avec: brew install claude"
    exit 1
fi
echo ""

# Étape 2: Compiler le serveur
echo -e "${YELLOW}ÉTAPE 2: Compilation du serveur MCP${NC}"
echo "-----------------------------------"
make clean && make build
if [ -f "./mcp-xlsm-server" ]; then
    echo -e "${GREEN}✓ Serveur compilé avec succès${NC}"
    chmod +x ./mcp-xlsm-server
else
    echo -e "${RED}✗ Échec de compilation${NC}"
    exit 1
fi
echo ""

# Étape 3: Test manuel du serveur en mode stdio
echo -e "${YELLOW}ÉTAPE 3: Test manuel en mode stdio${NC}"
echo "----------------------------------"
echo "Test du protocole MCP..."

# Test avec une séquence complète
TEST_SEQUENCE=$(cat <<'EOF'
{"jsonrpc":"2.0","method":"initialize","params":{"clientInfo":{"name":"test","version":"1.0.0"},"protocolVersion":"2024-11-05"},"id":1}
{"jsonrpc":"2.0","method":"list_tools","params":{},"id":2}
EOF
)

RESPONSE=$(echo "$TEST_SEQUENCE" | ./mcp-xlsm-server --stdio --config config.yaml 2>/dev/null)

if echo "$RESPONSE" | grep -q "analyze_file"; then
    echo -e "${GREEN}✓ Serveur répond correctement aux commandes MCP${NC}"
    echo "  Tools disponibles: analyze_file, build_navigation_map, query_data"
else
    echo -e "${RED}✗ Le serveur ne répond pas correctement${NC}"
    echo "Response: $RESPONSE"
    exit 1
fi
echo ""

# Étape 4: Création du script d'installation MCP
echo -e "${YELLOW}ÉTAPE 4: Création du script d'installation MCP${NC}"
echo "----------------------------------------------"
cat > install-mcp.sh <<'INSTALL_SCRIPT'
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
INSTALL_SCRIPT

chmod +x install-mcp.sh
echo -e "${GREEN}✓ Script d'installation créé: install-mcp.sh${NC}"
echo ""

# Étape 5: Instructions pour l'utilisateur
echo -e "${BLUE}=== INSTRUCTIONS D'UTILISATION ===${NC}"
echo ""
echo -e "${GREEN}1. INSTALLATION DU SERVEUR MCP:${NC}"
echo "   ./install-mcp.sh"
echo ""
echo -e "${GREEN}2. TEST RAPIDE (sans Claude Code):${NC}"
echo "   ./test_protocol.sh"
echo ""
echo -e "${GREEN}3. TEST DANS CLAUDE CODE:${NC}"
echo "   Après installation, demandez à Claude:"
echo "   - 'Analyse le fichier /Volumes/SSD/MCP/COMBINE INTERACTIF 02 2025.xlsm'"
echo "   - 'Montre-moi les données de la feuille FROUDIS'"
echo "   - 'Compare les données entre FROUDIS et CHAMDIS'"
echo ""
echo -e "${GREEN}4. DÉSINSTALLATION:${NC}"
echo "   claude mcp remove mcp-xlsm --scope user"
echo ""
echo -e "${GREEN}5. LOGS DE DEBUG:${NC}"
echo "   Les logs du serveur MCP sont dans ~/.claude/logs/"
echo ""

# Étape 6: Créer un test JSON-RPC pour debug
echo -e "${YELLOW}ÉTAPE 6: Création de tests JSON-RPC${NC}"
echo "------------------------------------"
cat > test_jsonrpc.txt <<'JSONRPC'
# Test 1: Initialize
{"jsonrpc":"2.0","method":"initialize","params":{"clientInfo":{"name":"test","version":"1.0.0"},"protocolVersion":"2024-11-05"},"id":1}

# Test 2: List tools
{"jsonrpc":"2.0","method":"list_tools","params":{},"id":2}

# Test 3: Analyze file
{"jsonrpc":"2.0","method":"analyze_file","params":{"path":"/Volumes/SSD/MCP/COMBINE INTERACTIF 02 2025.xlsm","include_patterns":true},"id":3}

# Test 4: Build navigation
{"jsonrpc":"2.0","method":"build_navigation_map","params":{"path":"/Volumes/SSD/MCP/COMBINE INTERACTIF 02 2025.xlsm","max_depth":2},"id":4}

# Test 5: Query data
{"jsonrpc":"2.0","method":"query_data","params":{"path":"/Volumes/SSD/MCP/COMBINE INTERACTIF 02 2025.xlsm","query":"FROUDIS","limit":5},"id":5}
JSONRPC

echo -e "${GREEN}✓ Tests JSON-RPC créés dans test_jsonrpc.txt${NC}"
echo "  Usage: cat test_jsonrpc.txt | grep -v '^#' | ./mcp-xlsm-server --stdio --config config.yaml"
echo ""

echo -e "${BLUE}=== TEST D'INTÉGRATION TERMINÉ ===${NC}"
echo -e "${GREEN}✓ Le serveur MCP est prêt à être installé${NC}"
echo ""
echo "Prochaine étape: ./install-mcp.sh"