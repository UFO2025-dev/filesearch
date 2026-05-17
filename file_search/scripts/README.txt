FileSearch v1.0 — Moteur de recherche local
============================================

INSTALLATION
------------
1. Double-cliquez sur scripts\install.bat
2. Répondez o/n pour le démarrage automatique
3. Un raccourci "FileSearch" apparaît sur votre bureau

UTILISATION
-----------
- Double-cliquez sur le raccourci "FileSearch" sur le bureau
- Le navigateur s'ouvre sur http://localhost:8080
- Tapez votre recherche dans le champ de recherche

PARAMÈTRES (page Paramètres dans l'UI)
---------------------------------------
- Ajouter un dossier à indexer
- Changer le mode de recherche (Auto / Essentiel / Avancé / Pro)
- Voir le nombre de fichiers indexés

MODES DE RECHERCHE
------------------
  Auto       — Mode détecté automatiquement selon le matériel
  Essentiel  — Recherche par mots-clés simple (rapide)
  Avancé     — Recherche FTS5 + filtres
  Pro        — Recherche sémantique IA (nécessite Ollama)

DÉMARRAGE AUTOMATIQUE
---------------------
Si activé lors de l'installation, FileSearch démarre avec Windows.
Pour désactiver : supprimez le fichier
  %APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup\FileSearch.bat

DÉSINSTALLATION
---------------
Lancez : scripts\install.bat --uninstall

LOGS
----
Les logs du serveur sont dans : %USERPROFILE%\FileSearch\data\server.log

COMMANDES MANUELLES
-------------------
Démarrer  : scriptsilesearch.bat
Arrêter   : scriptsilesearch.bat --kill

PRÉREQUIS
---------
- Windows 10/11 avec WSL2 (Ubuntu)
- WSL installé via Microsoft Store ou winget install wsl

SUPPORT
-------
Logs : ~/surveillance/gatewatch_mvp/file_search/data/server.log (dans WSL)
