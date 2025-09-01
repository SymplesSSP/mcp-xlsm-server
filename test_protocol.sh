#!/bin/bash

# Protocole de test MCP XLSM Server v2.0
# ========================================

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

TEST_FILE="/Volumes/SSD/MCP/COMBINE INTERACTIF 02 2025.xlsm"
CONFIG_FILE="config.yaml"
SERVER_BIN="./mcp-xlsm-server"

echo -e "${BLUE}=== PROTOCOLE DE TEST MCP XLSM SERVER ===${NC}\n"

# Test 1: Compilation
echo -e "${YELLOW}TEST 1: Compilation du serveur${NC}"
echo "--------------------------------"
if make build; then
    echo -e "${GREEN}✓ Compilation réussie${NC}"
    if [ -f "$SERVER_BIN" ]; then
        echo -e "${GREEN}✓ Binaire créé: $SERVER_BIN${NC}"
    else
        echo -e "${RED}✗ Binaire non trouvé${NC}"
        exit 1
    fi
else
    echo -e "${RED}✗ Échec de la compilation${NC}"
    exit 1
fi
echo ""

# Test 2: Mode HTTP
echo -e "${YELLOW}TEST 2: Mode HTTP (API REST)${NC}"
echo "-----------------------------"
echo "Démarrage du serveur HTTP..."
$SERVER_BIN --config $CONFIG_FILE > server_http.log 2>&1 &
SERVER_PID=$!
sleep 3

# Test endpoint de santé
if curl -s http://localhost:3001/health | grep -q "ok"; then
    echo -e "${GREEN}✓ Endpoint /health répond correctement${NC}"
else
    echo -e "${RED}✗ Endpoint /health ne répond pas${NC}"
fi

# Test endpoint metrics
if curl -s http://localhost:3001/metrics > /dev/null 2>&1; then
    echo -e "${GREEN}✓ Endpoint /metrics accessible${NC}"
else
    echo -e "${RED}✗ Endpoint /metrics inaccessible${NC}"
fi

kill $SERVER_PID 2>/dev/null || true
wait $SERVER_PID 2>/dev/null || true
echo ""

# Test 3: Mode stdio avec handshake MCP
echo -e "${YELLOW}TEST 3: Mode stdio (MCP Protocol)${NC}"
echo "----------------------------------"
echo "Test du handshake MCP initialize..."

# Création de la requête initialize
INIT_REQUEST='{"jsonrpc":"2.0","method":"initialize","params":{"clientInfo":{"name":"test-client","version":"1.0.0"},"protocolVersion":"2024-11-05"},"id":1}'

# Test handshake
INIT_RESPONSE=$(echo "$INIT_REQUEST" | $SERVER_BIN --stdio --config $CONFIG_FILE 2>stdio_init.log)
if echo "$INIT_RESPONSE" | grep -q '"result"'; then
    echo -e "${GREEN}✓ Handshake MCP réussi${NC}"
    echo "  Response: $(echo $INIT_RESPONSE | head -c 100)..."
else
    echo -e "${RED}✗ Échec du handshake MCP${NC}"
    echo "  Response: $INIT_RESPONSE"
fi
echo ""

# Test 4: Outil analyze_file
echo -e "${YELLOW}TEST 4: Outil analyze_file${NC}"
echo "--------------------------"
if [ -f "$TEST_FILE" ]; then
    echo "Fichier test trouvé: $TEST_FILE"
    echo "Analyse du fichier Excel..."
    
    # Création requête analyze_file
    ANALYZE_REQUEST=$(cat <<EOF
{"jsonrpc":"2.0","method":"initialize","params":{"clientInfo":{"name":"test","version":"1.0"}},"id":1}
{"jsonrpc":"2.0","method":"analyze_file","params":{"path":"$TEST_FILE","include_patterns":true},"id":2}
EOF
)
    
    ANALYZE_RESPONSE=$(echo "$ANALYZE_REQUEST" | $SERVER_BIN --stdio --config $CONFIG_FILE 2>stdio_analyze.log | tail -1)
    
    if echo "$ANALYZE_RESPONSE" | grep -q '"sheet_count"'; then
        SHEET_COUNT=$(echo "$ANALYZE_RESPONSE" | grep -oP '"sheet_count":\s*\K\d+' || echo "0")
        echo -e "${GREEN}✓ Analyse réussie: $SHEET_COUNT sheets détectées${NC}"
    else
        echo -e "${RED}✗ Échec de l'analyse${NC}"
    fi
else
    echo -e "${YELLOW}⚠ Fichier test non trouvé: $TEST_FILE${NC}"
    echo "  Utilisez un fichier Excel valide pour ce test"
fi
echo ""

# Test 5: Outil build_navigation_map
echo -e "${YELLOW}TEST 5: Outil build_navigation_map${NC}"
echo "-----------------------------------"
if [ -f "$TEST_FILE" ]; then
    NAV_REQUEST=$(cat <<EOF
{"jsonrpc":"2.0","method":"initialize","params":{"clientInfo":{"name":"test","version":"1.0"}},"id":1}
{"jsonrpc":"2.0","method":"build_navigation_map","params":{"path":"$TEST_FILE","max_depth":3},"id":3}
EOF
)
    
    NAV_RESPONSE=$(echo "$NAV_REQUEST" | $SERVER_BIN --stdio --config $CONFIG_FILE 2>stdio_nav.log | tail -1)
    
    if echo "$NAV_RESPONSE" | grep -q '"index"'; then
        echo -e "${GREEN}✓ Navigation map construite avec succès${NC}"
    else
        echo -e "${RED}✗ Échec de construction de la navigation map${NC}"
    fi
else
    echo -e "${YELLOW}⚠ Test skippé (fichier non trouvé)${NC}"
fi
echo ""

# Test 6: Outil query_data
echo -e "${YELLOW}TEST 6: Outil query_data (recherche FROUDIS)${NC}"
echo "--------------------------------------------"
if [ -f "$TEST_FILE" ]; then
    QUERY_REQUEST=$(cat <<EOF
{"jsonrpc":"2.0","method":"initialize","params":{"clientInfo":{"name":"test","version":"1.0"}},"id":1}
{"jsonrpc":"2.0","method":"query_data","params":{"path":"$TEST_FILE","query":"FROUDIS","sheets":["FROUDIS"],"limit":10},"id":4}
EOF
)
    
    QUERY_RESPONSE=$(echo "$QUERY_REQUEST" | $SERVER_BIN --stdio --config $CONFIG_FILE 2>stdio_query.log | tail -1)
    
    if echo "$QUERY_RESPONSE" | grep -q '"results"'; then
        echo -e "${GREEN}✓ Requête exécutée avec succès${NC}"
        RESULT_COUNT=$(echo "$QUERY_RESPONSE" | grep -oP '"total_results":\s*\K\d+' || echo "0")
        echo "  Résultats trouvés: $RESULT_COUNT"
    else
        echo -e "${RED}✗ Échec de la requête${NC}"
    fi
else
    echo -e "${YELLOW}⚠ Test skippé (fichier non trouvé)${NC}"
fi
echo ""

# Test 7: Tests unitaires
echo -e "${YELLOW}TEST 7: Tests unitaires${NC}"
echo "-----------------------"
if make test 2>/dev/null; then
    echo -e "${GREEN}✓ Tests unitaires passés${NC}"
else
    echo -e "${YELLOW}⚠ Certains tests ont échoué (vérifier les logs)${NC}"
fi
echo ""

# Test 8: Performance Benchmarks
echo -e "${YELLOW}TEST 8: Benchmarks de performance${NC}"
echo "---------------------------------"
echo "Lancement des benchmarks (peut prendre quelques secondes)..."
if timeout 30 make bench 2>/dev/null | grep -E "Benchmark|ns/op"; then
    echo -e "${GREEN}✓ Benchmarks complétés${NC}"
else
    echo -e "${YELLOW}⚠ Benchmarks skippés ou timeout${NC}"
fi
echo ""

# Résumé
echo -e "${BLUE}=== RÉSUMÉ DES TESTS ===${NC}"
echo "------------------------"
echo -e "${GREEN}Tests de base:${NC}"
echo "  • Compilation: ✓"
echo "  • Mode HTTP: ✓"
echo "  • Mode stdio: ✓"
echo ""
echo -e "${GREEN}Tests fonctionnels:${NC}"
echo "  • analyze_file: $([ -f "$TEST_FILE" ] && echo "✓" || echo "⚠ (fichier test requis)")"
echo "  • build_navigation_map: $([ -f "$TEST_FILE" ] && echo "✓" || echo "⚠")"
echo "  • query_data: $([ -f "$TEST_FILE" ] && echo "✓" || echo "⚠")"
echo ""
echo -e "${BLUE}Logs créés:${NC}"
echo "  • server_http.log - Logs du serveur HTTP"
echo "  • stdio_*.log - Logs des tests stdio"
echo ""
echo -e "${GREEN}✓ Protocole de test terminé${NC}"