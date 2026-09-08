# Transport presse-papiers portable pour l’Output

Date : 2026-09-08  
Ticket : [Choisir le transport presse-papiers portable](https://github.com/theopoc/runny/issues/78)

## Décision

Utiliser un transport hybride derrière une petite interface injectable :

1. en session locale, essayer un outil natif présent sur la machine qui exécute Runny ;
2. sous tmux, envoyer le texte sur `stdin` à `tmux load-buffer -w -` ;
3. hors tmux en session distante (`SSH_CONNECTION` ou `SSH_TTY`), ne pas toucher au presse-papiers de l’hôte distant et envoyer `tea.SetClipboard(text)` ;
4. si aucun outil local compatible n’est disponible, ou s’il échoue, utiliser transport tmux puis OSC 52 direct selon environnement ;
5. transmettre le texte complet demandé en une seule opération, sans nouvelle troncature ni découpage OSC 52 ;
6. annoncer un succès seulement après sortie réussie d’un outil natif. Pour tmux ou OSC 52, annoncer « requête de copie envoyée » : aucun de ces chemins ne confirme que terminal extérieur a accepté écriture.

Ordre local recommandé :

- macOS : `pbcopy` ;
- Windows : `clip.exe` ;
- Linux/Unix : `wl-copy` si `WAYLAND_DISPLAY` est défini, sinon `xclip -selection clipboard` ou `xsel --clipboard --input` si `DISPLAY` est défini ;
- WSL sans session SSH : accepter `clip.exe` s’il est trouvé dans `PATH` après les outils Linux adaptés à la session graphique.

Chaque outil reçoit le contenu par `stdin`, via `exec.CommandContext`, sans shell et avec délai court. Nom, arguments et exécuteur doivent être injectables pour tests. Ne jamais écrire contenu copié dans logs ou message d’erreur.

Cette stratégie donne comportement confirmé pour copie locale et conserve seul mécanisme réellement portable vers presse-papiers du terminal client sous SSH. Adaptateur tmux fonctionne avec réglage sécurisé par défaut et évite parser une énorme séquence OSC venant de pane. Une stratégie OSC 52 seule serait plus petite, mais ne peut ni détecter refus du terminal ni garantir gros payload. Une stratégie outils natifs seule viserait mauvais presse-papiers sous SSH et échouerait souvent sur hôte distant sans environnement graphique.

## Faits établis

### Bubble Tea v2.0.8

Runny dépend de `charm.land/bubbletea/v2 v2.0.8`. Dans cette version, `tea.SetClipboard(s string) tea.Cmd` retourne un message interne ; boucle Bubble Tea le transforme ensuite en `ansi.SetSystemClipboard` et écrit séquence vers sortie terminal. Documentation source précise explicitement usage OSC 52 et absence de support universel : [Bubble Tea `clipboard.go` v2.0.8](https://github.com/charmbracelet/bubbletea/blob/v2.0.8/clipboard.go#L24-L34), [dispatch dans `tea.go` v2.0.8](https://github.com/charmbracelet/bubbletea/blob/v2.0.8/tea.go#L810-L815).

`github.com/charmbracelet/x/ansi v0.11.7`, dépendance effective, encode tout `s` en Base64 standard et produit une seule séquence `ESC ] 52 ; c ; <base64> BEL`. Aucun découpage, plafond, probe de capacité ou résultat d’acceptation n’existe dans ce chemin : [`ansi/clipboard.go` v0.11.7](https://github.com/charmbracelet/x/blob/ansi/v0.11.7/ansi/clipboard.go#L5-L32).

Conséquence : 4 MiB bruts deviennent 5 592 408 octets Base64 et 5 592 416 octets avec enveloppe émise ici. Coût mémoire et écriture terminal sont inévitables avec `tea.SetClipboard`. Plusieurs séquences standard ne constituent pas un découpage valide : chaque écriture OSC 52 remplace sélection. Runny doit donc envoyer une seule séquence ou choisir autre transport.

### SSH et tmux

OSC 52 traverse flux terminal SSH et agit côté terminal client ; tmux le décrit comme avantage majeur, sans X11 forwarding. Support reste conditionnel : terminal extérieur doit autoriser OSC 52, capacité `Ms` doit être connue, et `set-clipboard` doit permettre chemin : [wiki officielle tmux](https://github.com/tmux/tmux/wiki/Clipboard#how-it-works).

Pour application comme Runny exécutée *dans* tmux, valeur par défaut `set-clipboard external` interdit OSC 52 brut provenant de pane. Exiger `set-clipboard on` élargirait droits de toutes applications. `tmux load-buffer -w -` est meilleur chemin : option `-w` demande explicitement à tmux d’envoyer buffer vers presse-papiers terminal et fonctionne lorsque `set-clipboard` vaut `external` ou `on`, sous réserve capacité `Ms` et support terminal : [manuel tmux 3.6a](https://github.com/tmux/tmux/blob/3.6a/tmux.1#L7272-L7293), [définition de `set-clipboard`](https://github.com/tmux/tmux/blob/3.6a/options-table.c#L490-L497). tmux imbriqué exige toujours configuration correcte sur couches concernées : [tmux dans tmux](https://github.com/tmux/tmux/wiki/Clipboard#terminal-support---tmux-inside-tmux).

`allow-passthrough` ne doit pas devenir prérequis Runny : Bubble Tea ne produit pas enveloppe DCS passthrough et tmux sait générer OSC 52 nativement. Documenter `set-clipboard external|on` et diagnostic `tmux info | grep Ms` suffit pour chemin supporté.

### Support terminal et sécurité

Support/refus varie :

- Ghostty implémente OSC 52 et permet `clipboard-write = ask|allow|deny`; écriture est autorisée par défaut : [référence Ghostty](https://ghostty.org/docs/config/reference#clipboard-write), [protocole OSC 52 Ghostty](https://ghostty.org/docs/vt/osc/52).
- Alacritty expose `terminal.osc52 = Disabled|OnlyCopy|OnlyPaste|CopyPaste`, avec `OnlyCopy` par défaut : [manuel officiel Alacritty](https://github.com/alacritty/alacritty/blob/master/extra/man/alacritty.5.scd#terminal).
- WezTerm accepte écriture/effacement OSC 52 mais ignore requêtes de lecture : [séquences WezTerm](https://wezterm.org/escape-sequences.html#operating-system-command-sequences).
- Windows Terminal permet désactiver écriture OSC 52 avec `compatibility.allowOSC52`; option est vraie par défaut depuis v1.22 : [notes Windows Terminal v1.22](https://github.com/microsoft/terminal/discussions/18516). PowerShell documente aussi `Set-Clipboard -AsOSC52` précisément pour copier vers hôte local depuis session SSH : [Microsoft Learn](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.management/set-clipboard?view=powershell-7.5#example-3-copy-text-to-the-clipboard-of-the-local-host-over-an-ssh-remote-session).
- iTerm2 considère OSC 52 meilleur choix interopérable, mais exige consentement utilisateur pour lecture et écriture : [documentation iTerm2](https://iterm2.com/documentation-escape-codes.html).

Écriture OSC 52 permet à tout processus produisant sortie terminal d’écraser presse-papiers et facilite clipboard poisoning ; tmux avertit que tout programme capable d’écrire dans pane peut le faire quand option vaut `on` : [sécurité tmux](https://github.com/tmux/tmux/wiki/Clipboard#security-concerns). Ici action explicite `[y]` limite surprise, mais documentation doit signaler autorisation terminal. Runny n’a besoin d’aucune lecture OSC 52 ; ne pas appeler `tea.ReadClipboard`, car lecture exposerait contenu utilisateur et déclencherait permissions supplémentaires.

### Outils natifs et gros payloads

Outils locaux acceptent flux sur entrée standard : `clip` sous Windows ([Microsoft Learn](https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/clip)), `wl-copy` sous Wayland ([projet officiel](https://github.com/bugaevc/wl-clipboard)), `xclip` sous X11 ([projet officiel](https://github.com/astrand/xclip)). `xclip` documente mécanisme X11 `INCR` pour gros transferts, avantage face à séquence terminal monolithique.

OSC 52 ne définit pas plafond portable. Limites actuelles suffisent à montrer impossibilité de garantir buffer maximal :

- xterm limite parsing de chaîne à `maxStringParse = 600000` octets par défaut ; séquence plus longue est ignorée : [manuel xterm](https://invisible-island.net/xterm/manpage/xterm.html#VT100-Widget-Resources:maxStringParse) ;
- tmux 3.6a limite séquences de contrôle reçues d’un pane avec `input-buffer-size = 1048576` octets par défaut : [manuel](https://github.com/tmux/tmux/blob/3.6a/tmux.1#L4248-L4251), [constante source](https://github.com/tmux/tmux/blob/3.6a/tmux.h#L2979).

`tea.SetClipboard` sur 4 MiB dépasse largement deux valeurs. `tmux load-buffer -w -` contourne plafond du parser entrant tmux, mais terminal extérieur garde sa propre limite. Bubble Tea n’effectue aucun probe ni ACK. Ne pas inventer seuil universel, ne pas tronquer silencieusement, ne pas découper en séquences standard. Pour copie distante volumineuse : envoyer payload entier en best effort, afficher taille et statut non confirmé. Si terminal refuse, proposer diagnostic/configuration, pas fallback vers presse-papiers distant qui viserait autre machine.

## Contrat d’implémentation proposé

```go
type clipboardResult struct {
	Transport string // native or osc52
	Confirmed bool   // true only after native helper exits successfully
	Err       error
}

type clipboardWriter interface {
	Write(text string, env map[string]string) tea.Cmd
}
```

Comportement :

- résultat natif réussi : `Copied N bytes` ;
- outil absent/échec local : retour modèle déclenche transport tmux si `TMUX` est défini, sinon `tea.SetClipboard(text)` ;
- session SSH sous tmux : `tmux load-buffer -w -`, sans tentative d’outil de bureau distant ;
- session SSH hors tmux : OSC 52 direct, sans tentative `pbcopy`, `wl-copy`, `xclip`, `xsel` ou `clip.exe` distante ;
- ne jamais conserver seconde copie du payload plus longtemps que commande ; ne jamais inclure payload dans `error`, log ou télémétrie ;
- conserver marqueur `[runny: output truncated]` déjà présent dans source sélectionnée ; transport ne modifie pas texte.

Pour éviter blocage TUI, exécution native reste dans `tea.Cmd`, avec contexte borné. Ne pas passer par `tea.ExecProcess` si cela suspend terminal : outil sans interaction reçoit seulement `stdin` et retourne message de résultat.

## Preuves à exiger

Tests unitaires injectés :

- choix `pbcopy`, `clip.exe`, `wl-copy`, `xclip`/`xsel` selon OS, environnement et disponibilité ;
- présence `SSH_CONNECTION` ou `SSH_TTY` court-circuite tous outils natifs ; `TMUX` choisit `tmux load-buffer -w -` ;
- succès natif est confirmé ; absence, timeout ou code non nul produit repli tmux/OSC 52 et statut non confirmé ;
- payload Unicode, multilignes et payload de 4 MiB arrivent intégralement sur `stdin` natif ;
- payload tmux arrive intégralement sur `stdin`, sans interpolation shell, avec arguments exacts `load-buffer -w -` ;
- chemin OSC 52 reçoit texte intégral en une commande, sans logs contenant contenu ;
- `y` reste non bloquant pendant streaming.

Validation terminal réelle : Ghostty local avec écriture autorisée/refusée ; SSH direct ; tmux avec défaut `set-clipboard external`, puis `off`, et `Ms` présent ; payload court puis proche de 4 MiB. Résultat attendu en refus : aucun faux message de succès, TUI reste utilisable.

## Hors décision

Ce ticket ne définit ni représentation du texte copié, ni sélection souris/clavier, ni politique de cycle de vie sélection pendant streaming/resize/changement de cible. Il fixe seulement transport et sémantique succès/fallback.
