# MCP XLSM Server v2.0

Universal Excel/XLSM file analyzer for Claude Code using the Model Context Protocol (MCP)

## 🚀 Fonctionnalités

- **Analyse universelle** de fichiers XLSM sans logique métier
- **Chunking automatique** avec streaming pour gros fichiers
- **Indexation multi-niveaux** (BTree, Inverted, Spatial, Bloom Filter)
- **Gestion précise des tokens** avec tiktoken-go
- **Cache intelligent** avec hot data tracking
- **Curseurs opaques MCP** avec versioning
- **Compression adaptative** multi-niveaux
- **Monitoring complet** Prometheus + Jaeger

## 📋 Prérequis

- Go 1.21+
- Docker (optionnel)
- Fichiers XLSM à analyser

## 🛠 Installation

### Compilation locale

```bash
# Cloner le projet
git clone <repository-url>
cd mcp-xlsm-server

# Installer les dépendances
make deps

# Compiler
make build

# Lancer
./mcp-xlsm-server
```

### Docker

```bash
# Construire l'image
make docker-build

# Lancer le conteneur
make docker-run
```

## 🔧 Configuration

Le serveur utilise le fichier `config.yaml` :

```yaml
server:
  host: "0.0.0.0"
  port: 3000
  max_file_size: "500MB"
  max_concurrent_requests: 10

performance:
  worker_pool_size: 8
  stream_threshold: "10MB"

cache:
  max_memory: "100MB"
  default_ttl: 5m

monitoring:
  prometheus:
    enabled: true
    port: 9090
```

Variables d'environnement :
- `CONFIG_PATH` : Chemin vers le fichier de config
- `LOG_LEVEL` : Niveau de log (debug, info, warn, error)

## 📡 API MCP

### Tool 1: `analyze_file`

Analyse les métadonnées et structure avec chunking automatique.

```json
{
  "method": "analyze_file",
  "params": {
    "filepath": "/path/to/file.xlsm",
    "chunk_size": 50,
    "stream_mode": true
  }
}
```

**Réponse :**
```json
{
  "result": {
    "metadata": {
      "checksum": "sha256...",
      "file_size": 524288000,
      "sheets_count": 244,
      "complexity_score": 7.5
    },
    "chunks": [
      {
        "chunk_id": "chunk_0_49",
        "sheets_range": [0, 49],
        "streaming_required": true,
        "cursor": "eyJjaHVua19pZCI6..."
      }
    ],
    "token_management": {
      "model_detected": "sonnet-4",
      "limits": {
        "context": 200000,
        "safe_buffer": 180000
      }
    }
  }
}
```

### Tool 2: `build_navigation_map`

Construit un index navigable avec pagination.

```json
{
  "method": "build_navigation_map",
  "params": {
    "filepath": "/path/to/file.xlsm",
    "checksum": "sha256...",
    "window_size": 1000
  }
}
```

### Tool 3: `query_data`

Requête multi-feuilles avec fenêtrage.

```json
{
  "method": "query_data",
  "params": {
    "query": "recherche texte",
    "navigation_index": {...},
    "window_config": {
      "max_results": 100,
      "max_rows_per_sheet": 1000
    }
  }
}
```

## 🔍 Monitoring

### Endpoints de santé

- `GET /health` - État du serveur
- `GET /metrics` - Métriques Prometheus

### Métriques disponibles

- `request_duration_seconds` - Durée des requêtes
- `token_usage_total` - Utilisation des tokens
- `cache_hit_ratio` - Ratio de hit cache
- `memory_usage_bytes` - Utilisation mémoire
- `index_rebuild_total` - Nombre de rebuilds d'index

## 🧪 Tests

```bash
# Tests unitaires
make test

# Tests avec couverture
make coverage

# Tests d'intégration
make test-integration

# Benchmarks
make bench
```

## 🚀 Déploiement

### Production

```bash
# Construire pour production
make release

# Déployer avec Docker
docker run -d \
  -p 3000:3000 \
  -p 9090:9090 \
  -v /path/to/config.yaml:/app/config.yaml \
  -v /path/to/logs:/var/log/mcp-xlsm \
  mcp-xlsm-server:latest
```

### Kubernetes

```bash
# Déployer sur K8s
make k8s-deploy
```

## 📊 Performance

### Métriques cibles

| Opération | Cible | Max Acceptable |
|-----------|-------|----------------|
| Analyse 244 feuilles | < 3s | < 5s |
| Construction index | < 7s | < 10s |
| Requête avec index | < 300ms | < 500ms |
| Fenêtre 1000 lignes | < 500ms | < 1s |

### Optimisations

- **Streaming** pour fichiers > 10MB
- **Cache intelligent** avec hot data
- **Index incrémental** avec delta updates
- **Compression adaptative** selon tokens

## 🛡 Sécurité

- Validation taille fichiers
- Rate limiting par endpoint
- Sanitization des formulas
- Logs sécurisés (pas de données sensibles)

## 🔧 Développement

### Setup environnement

```bash
# Setup outils de dev
make dev-setup

# Hot reload
make dev-watch

# Linting
make lint

# Format code
make fmt
```

### Architecture

```
internal/
├── server/       # Serveur HTTP et handlers MCP
├── models/       # Types et structures de données
├── cursor/       # Gestion curseurs opaques
├── token/        # Comptage précis des tokens
├── cache/        # Cache intelligent
├── index/        # Indexation multi-niveaux
├── streaming/    # Support streaming
└── compression/  # Compression adaptative
```

## 📚 Documentation

- [Guide API](docs/api.md)
- [Guide déploiement](docs/deployment.md)
- [Guide performance](docs/performance.md)
- [Troubleshooting](docs/troubleshooting.md)

## 🤝 Contribution

1. Fork le projet
2. Créer une branche feature
3. Commit avec tests
4. Push et créer PR

### Standards

- Tests obligatoires pour nouvelles features
- Couverture > 80%
- Linting clean
- Documentation à jour

## 📝 Changelog

### v2.0.0
- ✅ Curseurs opaques MCP
- ✅ Streaming pour gros fichiers
- ✅ Token counting précis
- ✅ Cache intelligent
- ✅ Index multi-niveaux
- ✅ Compression adaptative
- ✅ Monitoring complet

## 📄 License

MIT License - voir [LICENSE](LICENSE)

## 🆘 Support

- Issues: [GitHub Issues](https://github.com/user/mcp-xlsm-server/issues)
- Documentation: [Wiki](https://github.com/user/mcp-xlsm-server/wiki)
- Email: support@example.com

## 🏆 Acknowledgments

- [Excelize](https://github.com/xuri/excelize) pour manipulation XLSM
- [tiktoken-go](https://github.com/pkoukk/tiktoken-go) pour comptage tokens
- [Prometheus](https://prometheus.io/) pour monitoring
- [Brotli](https://github.com/andybalholm/brotli) pour compression