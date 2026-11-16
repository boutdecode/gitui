# gitui

Un client Git en terminal, construit avec la stack [Charm](https://charm.land)
(Bubble Tea v2, Bubbles, Lip Gloss). Premier provider : **GitHub**.

- Dépôts, pull requests et issues, avec filtres (à relire, assignées, les miennes)
- Détail avec conversation défilante et diff coloré
- Commenter, approuver, merger, ouvrir dans le navigateur
- Connexion par OAuth *device flow*, token stocké dans le trousseau du système
- Thème clair/sombre automatique, raccourcis configurables

## Installation

Il faut Go ≥ 1.26.

```sh
go install github.com/kevin/gitui/cmd/gitui@latest   # adaptez le chemin du module
# ou, depuis un clone :
make build        # produit bin/gitui
```

## Premier lancement

1. Créez une **OAuth App** GitHub (*Settings → Developer settings → OAuth Apps*)
   et cochez **Enable Device Flow**. Aucun secret n'est nécessaire : le client ID
   est public.
2. Renseignez son client ID, au choix :
   - variable d'environnement `GITUI_CLIENT_ID`
   - `client_id = "…"` dans le fichier de configuration (ci-dessous)
3. Lancez `gitui`, ouvrez l'URL affichée et saisissez le code (`c` le copie).

Pour essayer l'interface sans compte : `gitui --fake`.

Le token est gardé dans le trousseau du système. Sans trousseau (Linux sans
session graphique), il est écrit dans `~/.config/gitui/token` en mode 0600.
`L` déconnecte et supprime le token.

## Configuration

`~/.config/gitui/config.toml` (le dossier suit `os.UserConfigDir()`) :

```toml
client_id = "Iv1.xxxxxxxxxxxx"
scopes    = ["repo", "read:org"]   # défaut
theme     = "auto"                 # auto | light | dark

# La table [keys] doit venir après les clés ci-dessus.
[keys]
merge = ["M"]
quit  = ["q", "x"]
```

`GITUI_CLIENT_ID` l'emporte sur `client_id`. `theme = "auto"` suit la couleur de
fond du terminal.

## Raccourcis

Toutes les actions ci-dessous sont reconfigurables dans `[keys]`. Un nom inconnu,
une liste vide ou une touche utilisée deux fois sur un même écran fait échouer le
démarrage avec un message explicite. `ctrl+c` quitte toujours, quoi qu'on
configure. Les touches des composants (déplacement dans les listes, défilement,
édition de texte) ne sont pas configurables.

| Action    | Défaut            | Où                              |
|-----------|-------------------|---------------------------------|
| `quit`    | `q`, `ctrl+c`     | partout                         |
| `help`    | `?`               | partout                         |
| `back`    | `esc`, `backspace`| partout                         |
| `logout`  | `L`               | une fois connecté               |
| `open`    | `enter`           | listes                          |
| `switch`  | `tab`             | PR ↔ issues                     |
| `browser` | `o`               | listes et détail                |
| `search`  | `/`               | listes                          |
| `filter`  | `f`               | liste des PR                    |
| `comment` | `c`               | détail                          |
| `approve` | `a`               | détail d'une PR ouverte         |
| `merge`   | `m`               | détail d'une PR ouverte         |
| `diff`    | `tab`             | détail d'une PR                 |
| `send`    | `ctrl+s`, `alt+enter` | éditeur de commentaire      |
| `cancel`  | `esc`             | éditeur de commentaire          |
| `yes`     | `y`               | confirmation                    |
| `no`      | `n`, `esc`        | confirmation                    |
| `copy`    | `c`               | écran de connexion              |
| `retry`   | `r`               | écran de connexion              |

## Limites connues

- Les descriptions et commentaires sont affichés en texte brut (pas de markdown).
- Seuls les commentaires de conversation sont listés, pas ceux attachés au diff.
- L'approbation n'envoie pas de message ; le merge utilise la méthode par défaut.
- Les listes sont plafonnées à 1000 éléments (10 pages de 100).

## Architecture

```
cmd/gitui            point d'entrée
internal/domain      types neutres (Repo, PullRequest, Issue, …)
internal/provider    interface Provider ; github/ (réel) et fake/ (hors ligne)
internal/auth        device flow + stockage du token
internal/config      lecture de config.toml
internal/browser     ouverture d'URL
internal/tui         modèles Bubble Tea (une pile d'écrans)
```

L'interface ne dépend que de `internal/provider` : ajouter GitLab revient à
implémenter `Provider`. Les appels réseau passent uniquement par des `tea.Cmd`.

## Développement

```sh
make test      # go test ./...
make vet       # go vet ./...
make run-fake  # interface avec données factices
```
