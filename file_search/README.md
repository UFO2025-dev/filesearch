# FileSearch 🔍

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://go.dev)
[![Platform](https://img.shields.io/badge/Platform-Windows%20WSL2-blue?logo=windows)](https://learn.microsoft.com/windows/wsl/)

**Moteur de recherche local ultra-rapide pour vos fichiers** — propulsé par Go, SQLite FTS5 et la recherche sémantique IA (Ollama).

> Trouvez n'importe quel fichier sur votre machine en moins d'une seconde, même par sens, pas seulement par mot-clé.

---

## 📸 Aperçu

```
┌─────────────────────────────────────────────────────────────┐
│  🔍 FileSearch                          ⚙️ Paramètres        │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   ┌─────────────────────────────────────┐  [Rechercher]    │
│   │  rapport budget 2025...             │                   │
│   └─────────────────────────────────────┘                  │
│    Filtre: [Tous ▼]  Depuis: [─────]   🧠 Sémantique ON    │
│                                                             │
│   📄 rapport_budget_2025.pdf          C:\Users\...  [Ouvrir]│
│   📝 budget_notes.txt                 C:\Users\...  [Ouvrir]│
│   📊 previsions_2025.xlsx             C:\Users\...  [Ouvrir]│
│                                                             │
│   ✅ 3 résultats en 12ms  |  535 fichiers indexés  |  🟡 Avancé │
└─────────────────────────────────────────────────────────────┘
```

---

## ✨ Fonctionnalités

| Fonctionnalité | Description |
|---|---|
| ⚡ **Recherche FTS5** | Recherche plein-texte SQLite ultra-rapide |
| 🧠 **Recherche sémantique** | Embeddings via Ollama (nomic-embed-text) |
| 👁️ **Watcher temps réel** | Indexation automatique des nouveaux fichiers |
| 🗂️ **Multi-dossiers** | Indexez plusieurs répertoires simultanément |
| 🔒 **Sécurité** | Token d'authentification, path traversal bloqué |
| 📂 **Ouverture directe** | Ouvre les fichiers dans leur app Windows native |
| 🎨 **UI web** | Interface complète dans le navigateur |
| ⚙️ **Paramètres persistants** | Config JSON sauvegardée entre les redémarrages |
| 🔎 **Filtres** | Par extension (.pdf, .py, .go…) et par date |
| 📜 **Historique** | Les 10 dernières recherches |

---

## 🚀 Installation rapide (Windows + WSL2)

### Prérequis
- Windows 10/11 avec **WSL2** (Ubuntu)
- **Go 1.21+** dans WSL : `sudo apt install golang-go`
- (Optionnel) **Ollama** pour la recherche sémantique : https://ollama.ai

### Installation en 1 clic
```bat
:: Depuis Windows, double-cliquez sur :
file_search\scripts\install.bat
```

L'installateur :
1. Compile le binaire Go automatiquement si nécessaire
2. Crée `%USERPROFILE%\FileSearch\`
3. Crée un raccourci **FileSearch** sur le bureau
4. Propose le démarrage automatique avec Windows

### Démarrage manuel
```bat
file_search\scripts\filesearch.bat
```
→ Ouvre automatiquement `http://localhost:8080`

---

## 🛠️ Développement

```bash
cd file_search

# Lancer en mode développement
go run ./cmd/server -dir /mnt/c/Users/VotreNom/Documents

# Compiler le binaire
CGO_ENABLED=0 go build -ldflags="-s -w" -o filesearch-server ./cmd/server

# Tests
go test ./...
```

---

## 🔧 Modes de recherche

Le mode est détecté automatiquement selon le matériel, ou peut être forcé via l'UI :

| Mode | RAM | Sémantique | Cas d'usage |
|---|---|---|---|
| **Essentiel** | < 4 GB | ❌ | Machines légères |
| **Avancé** | 4–16 GB | ✅ | Usage quotidien |
| **Pro** | > 16 GB | ✅ | Workstations |

---

## 📁 Structure du projet

```
file_search/
├── cmd/server/        # Point d'entrée principal
├── internal/
│   ├── config/        # Config persistante (JSON)
│   ├── db/            # SQLite FTS5 + vecteurs
│   ├── embedder/      # Client Ollama + indexeur background
│   ├── indexer/       # Indexation des fichiers
│   ├── watcher/       # Surveillance fsnotify
│   ├── server/        # HTTP server + routes
│   │   └── static/    # UI web (index.html)
│   ├── security/      # Validation des chemins
│   └── cache/         # Cache LRU des résultats
├── scripts/
│   ├── install.bat    # Installateur Windows
│   └── filesearch.bat # Launcher Windows
└── start.sh           # Script de lancement WSL
```

---

## 🗺️ Roadmap

- [x] Recherche FTS5 SQLite
- [x] Recherche sémantique (Ollama)
- [x] Watcher temps réel
- [x] UI web complète
- [x] Config persistante
- [x] Installateur Windows
- [ ] Packaging MSI / NSIS
- [ ] Hotkey global `Ctrl+Space`
- [ ] Suppression de dossier dans les paramètres
- [ ] Support macOS / Linux natif

---

## 🤝 Contribuer

Les contributions sont les bienvenues ! Voir [CONTRIBUTING.md](CONTRIBUTING.md).

---

## 📄 Licence

[MIT](LICENSE) — Youssef (UFO2025-dev), 2026
