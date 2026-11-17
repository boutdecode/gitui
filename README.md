# gitui

Un client Git en terminal, construit avec la stack [Charm](https://charm.land)
(Bubble Tea v2, Bubbles, Lip Gloss). Premier provider : **GitHub**.

- Dépôts, pull requests et issues, avec filtres (à relire, assignées, les miennes)
- Détail avec conversation défilante et diff coloré
- Commenter, approuver, merger, ouvrir dans le navigateur
- Connexion par token d'accès personnel **ou** OAuth *device flow* (token OAuth
  stocké dans le trousseau du système)
- Thème clair/sombre automatique, raccourcis configurables

## Installation

Il faut Go ≥ 1.26.

```sh
go install github.com/kevin/gitui/cmd/gitui@latest   # adaptez le chemin du module
# ou, depuis un clone :
make build        # produit ./gitui à la racine
```

## Authentification

Deux modes, du plus prioritaire au moins prioritaire :

1. **Token d'accès personnel (PAT)** : variable `GITUI_TOKEN`, sinon
   `GITHUB_TOKEN`, sinon `token:` dans `config.yaml`. Aucune OAuth App n'est
   nécessaire. Le token est vérifié au démarrage ; il n'est **jamais** écrit dans
   le trousseau ni sur disque par gitui. S'il est refusé, gitui l'indique (avec sa
   source) au lieu de basculer sur OAuth ; corrigez-le puis relancez, ou `r`
   revérifie.
2. **OAuth device flow** : session stockée, ou nouvelle connexion (ci-dessous).

Pour un PAT classique, cochez les scopes `repo` et `read:org`. Pour un token
fine-grained, donnez (à titre indicatif) un accès en lecture/écriture aux
*Pull requests*, *Issues* et
*Contents* des dépôts voulus.

> ⚠️ Un `token:` dans `config.yaml` est stocké **en clair**. Préférez
> `GITUI_TOKEN` ; sinon gardez le fichier en mode 0600 (gitui avertit sinon).
> `GITHUB_TOKEN` est aussi lu car c'est la variable standard, mais elle sert
> alors à gitui : retirez-la de l'environnement si vous ne le souhaitez pas.

Avec un token fourni ainsi, `L` (déconnexion) est indisponible : il n'y a rien à
supprimer, retirez la variable ou la ligne `token:`.

### Réinitialiser l'authentification

```sh
gitui --logout
```

Supprime la session OAuth stockée (trousseau **et** fichier de repli) puis quitte,
sans ouvrir l'interface ni créer de config. Cela fonctionne même si `config.yaml`
est cassé. Un token donné par `GITUI_TOKEN`, `GITHUB_TOKEN` ou `token:` n'est pas
touché : la commande vous signale d'où il vient, car gitui continuera de l'utiliser
tant que vous ne l'aurez pas retiré. Le token n'est jamais affiché.

### OAuth device flow

1. Créez une **OAuth App** GitHub (*Settings → Developer settings → OAuth Apps*)
   et cochez **Enable Device Flow**. Aucun secret n'est nécessaire : le client ID
   est public.
2. Renseignez son client ID, au choix :
   - variable d'environnement `GITUI_CLIENT_ID`
   - `client_id: …` dans le fichier de configuration (ci-dessous)
3. Lancez `gitui`, ouvrez l'URL affichée et saisissez le code (`c` le copie).

Pour essayer l'interface sans compte : `gitui --fake`.

L'interface peint le fond du thème (violet très sombre, ou clair). `gitui --transparent` garde à la place le fond de votre terminal.

Le token OAuth est gardé dans le trousseau du système. Sans trousseau (Linux sans
session graphique), il est écrit dans `~/.config/gitui/token` en mode 0600.
`L` déconnecte et supprime ce token.

## Configuration

`~/.config/gitui/config.yaml` (le dossier suit `os.UserConfigDir()`). Il est créé
automatiquement au premier lancement, avec les valeurs par défaut commentées ;
un fichier existant n'est jamais modifié. Exemple :

```yaml
client_id: Iv1.xxxxxxxxxxxx   # OAuth uniquement
# token: ghp_…                # PAT, en clair : préférez GITUI_TOKEN
scopes: [repo, read:org]   # défaut
theme: auto                # auto | light | dark

keys:
  merge: [M]
  quit: [q, x]
```

`GITUI_CLIENT_ID` l'emporte sur `client_id`. Une clé inconnue (faute de frappe
comprise) est refusée au démarrage. `theme = "auto"` suit la couleur de
fond du terminal.

## Écrans

- **Dépôts** : onglets Mes dépôts / Favoris / Organisations / Récents, tableau à gauche, détail à droite. `enter` ouvre un dépôt.
- **Code** (`1`) : arbre de fichiers repliable et visionneuse avec coloration syntaxique.
- **Pull requests** (`2`) et **Issues** (`3`) : liste à gauche, aperçu à droite (checks CI, fichiers modifiés, commentaires). `enter` ouvre le détail plein écran (diff, commentaires, approbation, merge).

En dessous de 90 colonnes, les aperçus sont masqués et seule la liste est affichée.

Les dépôts ouverts récemment sont mémorisés dans `recent.json`, à côté du fichier de configuration.

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
| `switch`  | `tab`             | onglet suivant (`1`-`4` : accès direct) |
| `switchback` | `shift+tab`    | onglet précédent                |
| `browser` | `o`               | listes et détail                |
| `search`  | `/`               | listes                          |
| `filter`  | `f`               | dépôts (visibilité), PR et issues (état) |
| `scope`   | `p`               | liste des PR (tous / à relire / assignées / les miennes) |
| `star`    | `s`               | dépôts : ajouter/retirer des favoris |
| `sort`    | `t`               | dépôts : activité / nom / étoiles |
| `refresh` | `r`               | recharger la liste courante     |
| `branch`  | `b`               | code : branche suivante         |
| `parent`  | `h`               | code : dossier parent / replier |
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

- Les descriptions et commentaires sont affichés avec un style minimal (titres, puces), sans rendu markdown complet.
- Les fichiers de plus de 1 Mo et les fichiers binaires ne sont pas affichés dans l'onglet Code.
- Seuls les commentaires de conversation sont listés, pas ceux attachés au diff.
- L'approbation n'envoie pas de message ; le merge utilise la méthode par défaut.
- Les listes sont plafonnées à 1000 éléments (10 pages de 100).

## Architecture

```
cmd/gitui            point d'entrée
internal/domain      types neutres (Repo, PullRequest, Issue, …)
internal/provider    interface Provider ; github/ (réel) et fake/ (hors ligne)
internal/auth        device flow + stockage du token
internal/config      lecture de config.yaml
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
