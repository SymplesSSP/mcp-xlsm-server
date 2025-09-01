#!/bin/bash

echo "🚀 EXTRACTION DONNÉES FINANCIÈRES via MCP"
echo "=========================================="

# Fonction pour extraire les données d'une feuille
extract_sheet_data() {
    local sheet_name=$1
    echo "📊 Extraction $sheet_name..."
    
    curl -s -X POST http://localhost:3001/ \
        -H "Content-Type: application/json" \
        -d "{
            \"method\": \"query_data\",
            \"params\": {
                \"query\": \"$sheet_name\",
                \"navigation_index\": $(jq '.result.navigation_index' navigation_complete.json),
                \"window_config\": {
                    \"max_results\": 500,
                    \"max_rows_per_sheet\": 500,
                    \"max_sheets_per_call\": 1
                }
            },
            \"id\": \"extract-$sheet_name-data\"
        }" > "${sheet_name}_data.json"
    
    # Analyser les résultats
    if jq -e '.result.results.data[0].DataChunk' "${sheet_name}_data.json" >/dev/null 2>&1; then
        echo "✅ Données $sheet_name extraites"
        jq '.result.results.data[0].DataChunk | length' "${sheet_name}_data.json" | xargs echo "   Lignes:"
    else
        echo "❌ Échec extraction $sheet_name"
    fi
}

# Vérifier serveur MCP
if ! curl -s http://localhost:3001/health | jq -e '.status == "healthy"' >/dev/null; then
    echo "❌ Serveur MCP non disponible"
    exit 1
fi

echo "✅ Serveur MCP opérationnel"

# Extraire FROUDIS
extract_sheet_data "FROUDIS"

# Extraire CHAMDIS
extract_sheet_data "CHAMDIS"

echo ""
echo "📋 RÉSUMÉ DES EXTRACTIONS"
echo "========================"

if [ -f "FROUDIS_data.json" ]; then
    echo "📊 FROUDIS:"
    if jq -e '.result.results.data[0]' FROUDIS_data.json >/dev/null 2>&1; then
        jq -r '.result.results.data[0] | "   Location: \(.Location)\n   Window: \(.Window)\n   Données: \(.DataChunk | length) lignes"' FROUDIS_data.json
    else
        echo "   ❌ Aucune donnée extraite"
    fi
fi

if [ -f "CHAMDIS_data.json" ]; then
    echo "📊 CHAMDIS:"
    if jq -e '.result.results.data[0]' CHAMDIS_data.json >/dev/null 2>&1; then
        jq -r '.result.results.data[0] | "   Location: \(.Location)\n   Window: \(.Window)\n   Données: \(.DataChunk | length) lignes"' CHAMDIS_data.json
    else
        echo "   ❌ Aucune donnée extraite"
    fi
fi

echo ""
echo "✅ Extraction terminée"