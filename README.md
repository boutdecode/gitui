# gitui

A terminal Git client built with the [Charm](https://charm.land) stack
(Bubble Tea v2, Bubbles, Lip Gloss). First provider: **GitHub**.

- Repositories, pull requests and issues, with filters (to review, assigned, mine)
- Detail view with a scrollable conversation and a colored diff
- Comment, approve, merge, open in the browser
- Login with a personal access token **or** the OAuth *device flow* (the OAuth
  token is kept in the system keyring)
- Automatic light/dark theme, configurable shortcuts
- English and French interface

## Installation

Requires Go ≥ 1.26.

```sh
go install github.com/kevin/gitui/cmd/gitui@latest   # adjust the module path
# or, from a clone:
make build        # produces ./gitui at the root
```

## Authentication

Two modes, from highest to lowest priority:

1. **Personal access token (PAT)**: the `GITUI_TOKEN` variable, else
   `GITHUB_TOKEN`, else `token:` in `config.yaml`. No OAuth App is needed. The
   token is checked at startup; gitui **never** writes it to the keyring or to
   disk. If it is rejected, gitui says so (with its source) instead of falling
   back to OAuth; fix it and restart, or press `r` to check again.
2. **OAuth device flow**: a stored session, or a new login (below).

For a classic PAT, tick the `repo` and `read:org` scopes. For a fine-grained
token, give (as a guideline) read/write access to *Pull requests*, *Issues* and
*Contents* of the repositories you want.

> ⚠️ A `token:` in `config.yaml` is stored **in clear text**. Prefer
> `GITUI_TOKEN`; otherwise keep the file at mode 0600 (gitui warns you if not).
> `GITHUB_TOKEN` is also read because it is the standard variable, but gitui
> will then use it: remove it from the environment if you do not want that.

With a token provided this way, `L` (log out) is unavailable: there is nothing
to delete, remove the variable or the `token:` line instead.

### Resetting authentication

```sh
gitui --logout
```

Deletes the stored OAuth session (keyring **and** fallback file) and quits,
without opening the UI or creating a config. It works even if `config.yaml` is
broken. A token given by `GITUI_TOKEN`, `GITHUB_TOKEN` or `token:` is left
alone: the command tells you where it comes from, since gitui keeps using it
until you remove it. The token is never printed.

### OAuth device flow

1. Create a GitHub **OAuth App** (*Settings → Developer settings → OAuth Apps*)
   and tick **Enable Device Flow**. No secret is needed: the client ID is public.
2. Provide its client ID, either:
   - the `GITUI_CLIENT_ID` environment variable
   - `client_id: …` in the configuration file (below)
3. Run `gitui`, open the displayed URL and enter the code (`c` copies it).

To try the interface without an account: `gitui --fake`.

The interface paints the theme background (very dark purple, or light).
`gitui --transparent` keeps your terminal's background instead.

The OAuth token is kept in the system keyring. Without a keyring (Linux without
a graphical session) it is written to `~/.config/gitui/token` with mode 0600.
`L` logs out and deletes this token.

## Configuration

`~/.config/gitui/config.yaml` (the directory follows `os.UserConfigDir()`). It
is created automatically on first launch, with commented default values; an
existing file is never modified. Example:

```yaml
client_id: Iv1.xxxxxxxxxxxx   # OAuth only
# token: ghp_…                # PAT, in clear text: prefer GITUI_TOKEN
scopes: [repo, read:org]   # default
theme: auto                # auto | light | dark
lang: auto                 # auto (from the system) | en | fr

keys:
  merge: [M]
  quit: [q, x]
```

`GITUI_CLIENT_ID` takes precedence over `client_id`. An unknown key (typos
included) is rejected at startup. `theme: auto` follows the terminal's
background color.

## Language

The interface is available in English (the default) and French. The language
is chosen, in order of priority:

1. the `--lang en|fr` option (e.g. `gitui --lang fr`);
2. `lang:` in `config.yaml` (`auto`, `en` or `fr`);
3. the system language: the `LC_ALL`, `LC_MESSAGES`, `LANGUAGE` and `LANG`
   variables;
4. otherwise English.

The language of the config file created on first launch follows the same
rules. Any unsupported system language falls back to English.

## Screens

- **Repositories**: My repos / Favorites / Organizations / Recent tabs, table on the left, detail on the right. `enter` opens a repository.
- **Code** (`1`): collapsible file tree and viewer with syntax highlighting.
- **Pull requests** (`2`) and **Issues** (`3`): list on the left, preview on the right (CI checks, changed files, comments). `enter` opens the full-screen detail (diff, comments, approval, merge).

Below 90 columns the previews are hidden and only the list is shown.

Recently opened repositories are remembered in `recent.json`, next to the configuration file.

## Shortcuts

Every action below can be rebound under `keys`. An unknown name, an empty list
or a key used twice on the same screen makes startup fail with an explicit
message. `ctrl+c` always quits, whatever you configure. Widget keys (moving in
lists, scrolling, text editing) are not configurable.

| Action    | Default           | Where                           |
|-----------|-------------------|---------------------------------|
| `quit`    | `q`, `ctrl+c`     | everywhere                      |
| `help`    | `?`               | everywhere                      |
| `back`    | `esc`, `backspace`| everywhere                      |
| `logout`  | `L`               | once logged in                  |
| `open`    | `enter`           | lists                           |
| `switch`  | `tab`             | next tab (`1`-`4`: direct access) |
| `switchback` | `shift+tab`    | previous tab                    |
| `browser` | `o`               | lists and detail                |
| `search`  | `/`               | lists                           |
| `filter`  | `f`               | repositories (visibility), PRs and issues (state) |
| `scope`   | `p`               | PR list (all / to review / assigned / mine) |
| `star`    | `s`               | repositories: add/remove favorites |
| `sort`    | `t`               | repositories: activity / name / stars |
| `refresh` | `r`               | reload the current list         |
| `branch`  | `b`               | code: next branch               |
| `parent`  | `h`               | code: parent directory / collapse |
| `comment` | `c`               | detail                          |
| `approve` | `a`               | detail of an open PR            |
| `merge`   | `m`               | detail of an open PR            |
| `diff`    | `tab`             | PR detail                       |
| `send`    | `ctrl+s`, `alt+enter` | comment editor              |
| `cancel`  | `esc`             | comment editor                  |
| `yes`     | `y`               | confirmation                    |
| `no`      | `n`, `esc`        | confirmation                    |
| `copy`    | `c`               | login screen                    |
| `retry`   | `r`               | login screen                    |

## Known limitations

- Descriptions and comments are shown with minimal styling (headings, bullets), without full markdown rendering.
- Files larger than 1 MB and binary files are not displayed in the Code tab.
- Only conversation comments are listed, not those attached to the diff.
- Approving does not send a message; merging uses the default method.
- Lists are capped at 1000 items (10 pages of 100).

## Architecture

```
cmd/gitui            entry point
internal/domain      neutral types (Repo, PullRequest, Issue, …)
internal/provider    Provider interface; github/ (real) and fake/ (offline)
internal/auth        device flow + token storage
internal/config      config.yaml loading
internal/i18n        translations (English source strings, French catalog)
internal/browser     URL opening
internal/tui         Bubble Tea models (a stack of screens)
```

The UI only depends on `internal/provider`: adding GitLab means implementing
`Provider`. Network calls only go through `tea.Cmd`s.

### Translations

User-visible strings are written in English in the source and passed through
`i18n.T("…")`; the English text is the lookup key. The French translations live
in `internal/i18n/fr.go`. A test fails when a string passed to `T` has no
French entry. To add a language, add a catalog next to `fr.go` and register
its code in `i18n.Langs` / `i18n.Parse`.

## Development

```sh
make test      # go test ./...
make vet       # go vet ./...
make run-fake  # interface with fake data
```
