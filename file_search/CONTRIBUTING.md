# Contribuer à FileSearch

Merci de l'intérêt ! Toute contribution est la bienvenue.

## 🚀 Démarrage rapide

```bash
# 1. Fork ce repo sur GitHub, puis :
git clone https://github.com/VOTRE_USERNAME/filesearch.git
cd filesearch/file_search

# 2. Installer les dépendances
go mod download

# 3. Lancer les tests
go test ./...

# 4. Lancer le serveur en dev
go run ./cmd/server -dir /mnt/c/Users/VotreNom/Documents
```

## 🌿 Workflow de contribution

```
master          — branche stable
feat/xxx        — nouvelle fonctionnalité
fix/xxx         — correction de bug
docs/xxx        — documentation uniquement
```

1. **Créez une branche** depuis `master` :
   ```bash
   git checkout -b feat/ma-fonctionnalite
   ```

2. **Faites vos changements** en suivant les conventions ci-dessous

3. **Lancez les tests** avant de soumettre :
   ```bash
   go test ./...
   go build ./...
   ```

4. **Commitez** avec un message clair :
   ```
   feat: ajouter suppression de dossier dans les paramètres
   fix: corriger le watcher sur les chemins avec espaces
   docs: améliorer le README installation
   ```

5. **Ouvrez une Pull Request** vers `master`

## 📐 Conventions de code

- **Go standard** : `gofmt`, pas de lint warnings
- **Interfaces** : préférer les interfaces à l'injection directe
- **Tests** : tout nouveau code doit avoir un test
- **Logs** : utiliser `slog` (pas `fmt.Println`)
- **Erreurs** : toujours wrapper avec `fmt.Errorf("context: %w", err)`

## 🐛 Signaler un bug

Ouvrez une [Issue GitHub](https://github.com/UFO2025-dev/filesearch/issues) avec :
- La version (commit hash)
- Les logs du serveur (`data/server.log`)
- Les étapes pour reproduire

## 💡 Idées de contributions

| Priorité | Feature |
|---|---|
| 🔴 Haute | Suppression d'un dossier dans les paramètres |
| 🔴 Haute | Hotkey global `Ctrl+Space` |
| 🟡 Moyenne | Support macOS / Linux natif |
| 🟡 Moyenne | Packaging MSI Windows |
| 🟢 Facile | Plus d'extensions supportées dans le watcher |
| 🟢 Facile | Thème sombre / clair dans l'UI |

## 📄 Licence

En contribuant, vous acceptez que votre code soit sous licence [MIT](LICENSE).
