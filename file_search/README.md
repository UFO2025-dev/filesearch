# FileSearch 🔍

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://go.dev)
[![Platform](https://img.shields.io/badge/Platform-Windows%20WSL2-blue?logo=windows)](https://learn.microsoft.com/windows/wsl/)
[![Release](https://img.shields.io/github/v/release/UFO2025-dev/filesearch)](https://github.com/UFO2025-dev/filesearch/releases)

> 🔒 **Confidentialité** — tout tourne sur votre machine, aucune donnée envoyée en ligne  
> ⚡ **Vitesse** — résultats en moins de 50ms grâce à SQLite FTS5  
> 🧠 **IA locale** — recherche par sens via Ollama, sans abonnement cloud  

**FileSearch est un moteur de recherche local pour vos fichiers**, pensé pour les gens qui veulent retrouver un document en 1 seconde sans sacrifier leur vie privée.

---

## ⚡ Quick Start

```bat
:: 1. Téléchargez le zip de la release et extrayez-le
:: 2. Double-cliquez sur :
file_search\scripts\install.bat

:: 3. Cliquez sur le raccourci "FileSearch" sur votre bureau
:: 4. Tapez votre recherche → résultats instantanés
```

**C'est tout.** Le serveur démarre dans WSL, le navigateur s'ouvre sur `http://localhost:8080`.

> **Prérequis** : Windows 10/11 + [WSL2 Ubuntu](https://learn.microsoft.com/windows/wsl/install) + [Go 1.21+](https://go.dev/dl/)  
> Pour la recherche sémantique IA (optionnel) : [Ollama](https://ollama.ai) + `ollama pull nomic-embed-text`

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
| ⚡ **Recherche FTS5** | Recherche plein-texte SQLite, < 50ms |
| 🧠 **Recherche sémantique** | Trouvez par sens, pas seulement par mot-clé |
| 👁️ **Watcher temps réel** | Nouveaux fichiers indexés automatiquement |
| 🗂️ **Multi-dossiers** | Indexez plusieurs répertoires simultanément |
| 🔒 **100% local** | Zéro cloud, zéro télémétrie, vos données restent chez vous |
| 📂 **Ouverture directe** | Ouvre les fichiers dans leur app Windows native |
| ⚙️ **Paramètres persistants** | Config sauvegardée entre les redémarrages |
| 🔎 **Filtres** | Par extension (.pdf, .py, .go…) et par date |
| 📜 **Historique** | Les 10 dernières recherches |

---

## 🛠️ Développement

```bash
cd file_search
go run ./cmd/server -dir /mnt/c/Users/VotreNom/Documents
```

```bash
# Compiler le binaire optimisé
CGO_ENABLED=0 go build -ldflags="-s -w" -o filesearch-server ./cmd/server

# Tests
go test ./...
```

---

## 🔧 Modes de recherche

Détecté automatiquement selon votre matériel, ou forcé via l'UI :

| Mode | RAM requise | Sémantique IA | Cas d'usage |
|---|---|---|---|
| **Essentiel** | < 4 GB | ❌ | Machines légères |
| **Avancé** | 4–16 GB | ✅ | Usage quotidien |
| **Pro** | > 16 GB | ✅ | Workstations |

---

## 📁 Structure

```
file_search/
├── cmd/server/        # Point d'entrée
├── internal/
│   ├── config/        # Config persistante JSON
│   ├── db/            # SQLite FTS5 + vecteurs
│   ├── embedder/      # Client Ollama + indexeur background
│   ├── indexer/       # Lecture de 30+ types de fichiers
│   ├── watcher/       # Surveillance fsnotify temps réel
│   ├── server/        # HTTP + UI web embarquée
│   └── security/      # Anti path traversal
├── scripts/
│   ├── install.bat    # Installateur Windows
│   └── filesearch.bat # Launcher
└── start.sh
```

---

## 🗺️ Roadmap

- [x] Recherche FTS5 + sémantique
- [x] Watcher temps réel
- [x] UI complète + paramètres persistants
- [x] Installateur Windows
- [ ] Packaging MSI
- [ ] Hotkey global `Ctrl+Space`
- [ ] Support macOS / Linux natif

---

## 🤝 Contribuer

Les contributions sont les bienvenues ! Voir [CONTRIBUTING.md](CONTRIBUTING.md).

---

## 📄 Licence

Ce projet est sous licence **[MIT](LICENSE)** — libre d'utilisation, de modification et de distribution.  
Copyright © 2026 Youssef ([UFO2025-dev](https://github.com/UFO2025-dev))
