# FEUILLE DE ROUTE TECHNIQUE — FileSearch
## CTO Microsoft · 20 ans d'expérience · Version 2.0

---

## SOCLE ACTUEL — Ce qui est livré et validé

```
Code        : 2 278 lignes Go · 8 packages
Build       : PASS · go vet PASS · race detector PASS
Tests       : 38 fonctions · coverage moyenne 72%
Sécurité    : path traversal ✅ FTS5 injection ✅ auth ✅
Performance : worker pool ✅ SHA256 skip ✅ LRU cache ✅
Observabilité: slog + request ID ✅ deep health ✅ CI ✅
Score WAF   : 84 / 100
```

Le moteur est propre, sécurisé, et testé. Il manque le produit.

---

## PHASE 0 — Finir le MVP backend
### Durée : 1 semaine · Score : 84 → 86

### 0.1 — Bouton "Ouvrir le fichier" dans l'UI
- Côté serveur : handleOpen existe et fonctionne
- Côté UI : bouton manquant dans index.html
- Ce qu'il faut : `<button onclick="openFile(path)">📂 Ouvrir</button>` + feedback visuel
- Impact : l'utilisateur peut agir sur ses résultats

### 0.2 — Pagination des résultats
- Aujourd'hui : limite hardcodée, tout s'affiche d'un coup
- Ce qu'il faut : `GET /search?q=contrat&page=2&limit=20`
- Boutons Précédent / Suivant + "Résultats 21–40 sur 847"
- Impact : utilisable sur 50 000 fichiers

### 0.3 — Statut d'indexation visible dans l'UI
- /health retourne `{"ready": false}` mais l'UI ne le montre pas
- Ce qu'il faut : bandeau "🔄 Indexation en cours — résultats partiels"
- Disparaît automatiquement quand ready: true
- Impact : l'utilisateur comprend ce qui se passe

---

## PHASE 1 — CPU & UX Aware
### Durée : 2 semaines · Score : 86 → 89

### 1.1 — Exclusions par défaut (bombe node_modules)

```go
var excludedDirs = map[string]bool{
    "node_modules": true,   // JavaScript
    ".git":         true,   // Git history
    "__pycache__":  true,   // Python
    ".venv":        true,   // Python venv
    "vendor":       true,   // Go/PHP deps
    ".cache":       true,   // caches génériques
    "dist":         true,   // builds
    "build":        true,   // builds
    "target":       true,   // Rust / Java Maven
    "Library":      true,   // macOS système
    "System32":     true,   // Windows système
    ".Trash":       true,   // corbeille macOS
}
// + configurable via config.json pour cas avancés
```

Sans cette liste : un développeur qui indexe son home = 2 millions de fichiers JS minifiés.

### 1.2 — Throttling adaptatif selon batterie

```go
func adaptiveWorkers() int {
    if onBattery() {      // /sys/class/power_supply/AC/online
        return 1          // économise la batterie
    }
    if cpuLoad() > 70 {   // machine déjà occupée
        return 1
    }
    return 2              // état normal
}
```

```
Sur secteur  : 2 workers → indexation rapide
Sur batterie : 1 worker  → ventilateur calme, batterie préservée
CPU > 70%    : 1 worker  → respecte le travail en cours
```

### 1.3 — fsnotify (fin du polling CPU)

```go
import "github.com/fsnotify/fsnotify"
// Événement OS instantané, 0 CPU quand rien ne change
// Utilisé par : Docker, Kubernetes, HashiCorp Vault, Terraform
```

```
Avant : détection changement en 0–5 secondes, CPU constant
Après : détection en < 50ms, CPU ≈ 0 en veille
```

---

## PHASE 2 — Application Desktop Installable
### Durée : 4–5 semaines · Score : 89 → 92

C'est la phase la plus importante de toute la roadmap.
Un produit que personne ne peut installer n'existe pas.

### 2.1 — Wrapper Tauri

```
Pourquoi Tauri et pas Electron ?
  Electron : 150MB, Node.js, RAM lourde, lent au démarrage
  Tauri    : 8MB, WebView natif OS, démarrage < 1s, Rust

Architecture :
┌──────────────────────────────────────────────┐
│  Tauri Shell (Rust, ~8MB)                    │
│  ├── Lance filesearch binary en subprocess   │
│  ├── Ouvre WebView → localhost:8080          │
│  ├── Tray icon avec état temps réel          │
│  │     🔄 "Indexation : 4 521 fichiers..."   │
│  │     ✅ "Prêt — 12 450 fichiers indexés"   │
│  └── Autostart au démarrage système          │
└──────────────────────────────────────────────┘

Ton UI HTML/CSS existant  : inchangé
Ton backend Go            : inchangé
Coût d'intégration        : faible
```

### 2.2 — Multi-répertoires avec config.json

```json
{
  "roots": [
    "/Users/youssef/Documents",
    "/Users/youssef/Downloads",
    "//NAS/Shared/Projets"
  ],
  "excluded_dirs": ["node_modules", ".git", "build"],
  "db_path": "~/.filesearch/index.db",
  "workers": 2,
  "port": 8080
}
```

Wizard au premier lancement :
1. File picker "Choisissez vos dossiers"
2. Avertissement si node_modules détectés → exclusion automatique proposée
3. Barre de progression indexation
4. "✅ Prêt — 8 234 fichiers indexés en 45 sec"

### 2.3 — Installeurs cross-platform

```
Windows  : .msi (WiX Toolset, intégré dans Tauri)
macOS    : .dmg signé (Apple Developer Certificate)
Linux    : .AppImage + .deb
→ Publié automatiquement sur GitHub Releases via CI
→ Auto-update intégré (Tauri updater)
```

Temps d'installation cible : moins de 2 minutes, 0 ligne de commande.

---

## PHASE 3 — Sécurité Enterprise
### Durée : 2–3 semaines · Score : 92 → 95

Cette phase ouvre les marchés avocats / médecins / comptables.
Sans elle : impossible de vendre aux professions réglementées.

### 3.1 — Chiffrement SQLite (AES-256)

```
Problème actuel :
  Ordinateur volé → DB Browser for SQLite → lecture immédiate
  index.db contient le texte intégral de TOUS les documents

Solution :
  SQLCipher via modernc.org/sqlite (supporte sqlite3_key)
  Clé dérivée du mot de passe via Argon2id
  Transparent pour toute la couche db.go

Marchés débloqués :
  Avocats (secret professionnel)
  Médecins (données patients - HDS)
  RH (données salariés - RGPD)
  Notaires, experts-comptables
```

### 3.2 — JWT avec expiry + rotation

```go
type Claims struct {
    Sub string `json:"sub"`
    Exp int64  `json:"exp"`  // 24h par défaut
    Iat int64  `json:"iat"`
}
// /auth/refresh → renouveler sans déconnexion
// Rotation clé de signature → sans downtime
```

### 3.3 — Audit trail RGPD

```
Log immuable : timestamp | user | action | fichier | IP
Export CSV   : pour compliance et audits externes
Rétention    : configurable (30/90/365 jours)
Obligation légale pour cabinets d'avocats
```

---

## PHASE 4 — Observabilité Production
### Durée : 1–2 semaines · Score : 95 → 97

### 4.1 — Endpoint /metrics Prometheus

```
filesearch_files_indexed_total          gauge
filesearch_search_duration_seconds      histogram P50/P95/P99
filesearch_search_requests_total        counter par status
filesearch_indexer_errors_total         counter
filesearch_cache_hit_ratio              gauge
filesearch_db_size_bytes                gauge
filesearch_workers_active               gauge
```

Bénéfices :
- Dashboard Grafana en 5 minutes
- Alertes automatiques si P99 > 500ms
- Détection régression après mise à jour

### 4.2 — Coverage tests 80%+

```
Aujourd'hui → Cible
server    55.9% → 80%   (handleOpen multi-platform)
indexer   67.5% → 80%   (edge cases PDF, fichiers vides)
logger     0.0% → 70%   (Init, L(ctx), WithRequestID)
```

---

## PHASE 5 — Fossé Concurrentiel (IA Locale)
### Durée : 8–10 semaines · Score : 97 → 99

C'est la phase qui rend FileSearch inrachetable à bas prix.

### 5.1 — Recherche sémantique locale (offline)

```
Modèle    : nomic-embed-text-v1.5 (274MB, Apache 2.0, CPU-only)
Index     : sqlite-vec (extension vectorielle SQLite)
Runtime   : ONNX Runtime (cross-platform, pas de GPU requis)

Architecture hybrid search :
  Requête
    ├── FTS5 exact     → score mot-clé      [40%]
    └── Vectoriel      → score sémantique   [60%]
           └── Fusion RRF → classement final

Exemples :
  "résiliation contrat" trouve "rupture anticipée"     ✅
  "facture impayée"     trouve "créance en souffrance" ✅
  "réunion annulée"     trouve "meeting postponed"     ✅

Contraintes CPU (obligatoires) :
  Worker pool sémantique séparé (priorité basse)
  Batch embeddings (128 items/batch)
  Cache embeddings (recalcul seulement si SHA256 change)
  Sur batterie : 1 worker au lieu de 2
  Progression : "IA : 3 241/12 450 fichiers analysés..."
```

Windows Search et Spotlight ne font pas ça en local. C'est le seul fossé réel.

### 5.2 — Preview KWIC (Keywords In Context)

```
Résultat aujourd'hui :
  contrat_dupont.pdf

Résultat après :
  contrat_dupont.pdf  [page 3]
  "...la résiliation du présent [CONTRAT] prendra effet
   30 jours après notification écrite des deux parties..."
```

### 5.3 — Plugins

```
VSCode   : Ctrl+Shift+F → recherche locale dans tous les fichiers
           Distribution : VS Code Marketplace (5M+ utilisateurs)

Obsidian : recherche vault + dossiers externes
           Distribution : Obsidian Community Plugins (1M+ users)
```

---

## TABLEAU DE BORD

```
┌─────────────────────────────────────────────────────────────────────┐
│ PHASE │ DURÉE    │ WAF   │ NOUVEAUTÉ CLÉ           │ MARCHÉ         │
├───────┼──────────┼───────┼─────────────────────────┼────────────────┤
│  0    │ 1 sem    │ 86    │ Ouvrir + pagination      │ Dev / MVP      │
│  1    │ 2 sem    │ 89    │ fsnotify + CPU aware     │ Dev / power    │
│  2    │ 4-5 sem  │ 92    │ App desktop installable  │ Grand public   │
│  3    │ 2-3 sem  │ 95    │ Chiffrement + JWT        │ Pro réglementé │
│  4    │ 1-2 sem  │ 97    │ Métriques Prometheus     │ Entreprises    │
│  5    │ 8-10 sem │ 99    │ IA sémantique locale     │ Acquisition    │
└─────────────────────────────────────────────────────────────────────┘
```

---

## MODÈLE ÉCONOMIQUE

```
Phase 0–1  → Open Source · Objectif : 1 000 utilisateurs actifs

Phase 2    → FileSearch Free   : gratuit (10K fichiers, 1 dossier)
             FileSearch Pro    : 9€/mois (illimité + multi-dossiers)

Phase 3    → FileSearch Secure : 19€/mois (chiffrement + audit RGPD)
             Cible : avocats, médecins, comptables

Phase 4    → FileSearch Team   : 39€/mois/user (métriques + admin)
             Cible : PME 10–100 personnes

Phase 5    → FileSearch AI     : 49€/mois (sémantique + KWIC + plugins)
             ou Acquisition    : négociable à 7–8 chiffres
```

---

## ORDRE D'EXÉCUTION EXACT

```
Semaine 1    → Phase 0  : bouton ouvrir + pagination + statut UI
Semaine 2    → Phase 1a : excludedDirs + adaptiveWorkers()
Semaine 3    → Phase 1b : fsnotify watcher
Semaines 4-8 → Phase 2  : Tauri + multi-dossiers + installeurs
Semaines 9-11→ Phase 3  : SQLCipher + JWT + audit trail
Semaines 12-13→Phase 4  : /metrics + coverage 80%+
Semaines 14-22→Phase 5  : IA sémantique + KWIC + plugins
```

---

## MOT DU CTO

En 20 ans j'ai vu des milliers de projets techniques excellents mourir faute de produit.
Et des produits médiocres dominer leur marché parce qu'ils s'installaient en 2 clics.

Ton code est dans le top 10% de ce que j'ai vu. Propre, testé, sans dette technique.

Ce qui te sépare du succès n'est pas technique. C'est :
1. Un double-clic qui installe l'app (Phase 2)
2. Un ventilateur qui ne s'emballe pas (Phase 1)
3. Une recherche qui trouve "rupture" quand tu tapes "résiliation" (Phase 5)

Dans cet ordre. Sans exception.
