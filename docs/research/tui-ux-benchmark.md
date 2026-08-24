# Benchmark UX pour la TUI de Runny

Date de recherche : 2026-08-15

## Objet et méthode

Cette note compare Runny à des TUI et outils terminal matures qui couvrent un ou plusieurs besoins voisins : navigation maître/détail, sélection multiple, suivi de processus parallèles, consultation de logs et exécution d'actions sensibles.

Les faits ci-dessous viennent uniquement de sources primaires : documentation et code officiels, changelogs maintenus par les projets, puis issues déposées dans leurs dépôts officiels. Les issues prouvent qu'un problème a été rapporté ; elles ne mesurent ni sa fréquence ni sa prévalence. Les recommandations pour Runny sont explicitement présentées comme des inférences.

Les nombres d'étoiles GitHub constituent seulement un indice d'adoption, jamais une preuve de qualité UX. Au 2026-08-15, l'API GitHub indiquait environ 82,5 k étoiles pour [fzf](https://api.github.com/repos/junegunn/fzf), 81,4 k pour [lazygit](https://api.github.com/repos/jesseduffield/lazygit), 44,4 k pour [Bubble Tea](https://api.github.com/repos/charmbracelet/bubbletea), 34,4 k pour [K9s](https://api.github.com/repos/derailed/k9s), 34,0 k pour [btop](https://api.github.com/repos/aristocratos/btop), 8,8 k pour [Bubbles](https://api.github.com/repos/charmbracelet/bubbles), 8,2 k pour [htop](https://api.github.com/repos/htop-dev/htop) et 2,7 k pour [dekit, successeur de mprocs](https://api.github.com/repos/pvolok/dekit).

## Verdict

Runny devrait conserver son modèle maître/détail « cibles à gauche, sortie à droite », mais alléger fortement son chrome et rendre quatre choses impossibles à manquer :

1. commande exacte et portée du prochain lancement ;
2. panneau qui possède le focus ;
3. différence entre curseur, sélection et état d'exécution ;
4. résumé textuel permanent `queued / running / ok / failed / cancelled`.

La meilleure direction n'est pas une copie graphique de lazygit ou K9s. C'est une interface plus sobre, centrée sur une table de cibles et un lecteur de logs, avec palette recherchable, aide contextuelle courte et mode mono réellement fonctionnel.

## Ce qui fonctionne dans les références

### 1. Hiérarchie maître/détail et focus

- Lazygit expose une navigation cohérente entre panneaux : `tab`, `shift-tab`, flèches et `h/j/k/l`, plus des accès directs numérotés. Ses raccourcis sont configurables et la recherche commence avec `/` ([configuration officielle](https://github.com/jesseduffield/lazygit/blob/master/docs/Config.md#default)).
- Le mode arbre de tmux associe une liste hiérarchique et un aperçu, avec `Left`/`Right` pour replier/déplier, `Up`/`Down` pour naviguer, `Enter` pour choisir et `v` pour masquer/afficher l'aperçu ([guide officiel tmux](https://github.com/tmux/tmux/wiki/Getting-Started#choosing-sessions-windows-and-panes)).
- mprocs sépare liste des processus et terminal du processus sélectionné. Il fournit un raccourci de bascule de focus, un zoom du terminal et des keymaps distinctes selon le panneau actif ([README officiel, keymap](https://github.com/pvolok/dekit/blob/master/README-mprocs.md#default-keymap)).

**Inférence Runny.** Deux panneaux restent adaptés. Le focus doit toutefois être indiqué par au moins deux signaux indépendants : titre explicite `TASKS — FOCUSED` ou `OUTPUT — FOCUSED`, plus bordure ou inversion. Une couleur de bordure seule ne suffit pas. `tab` et `shift-tab` devraient avancer/reculer entre panneaux ; `z` peut conserver le zoom.

### 2. Responsive : conserver le contexte avant le décor

- fzf peut utiliser une hauteur adaptative et possède une syntaxe de layout alternatif sous un seuil de taille, par exemple passer l'aperçu de droite vers le haut lorsqu'il devient trop petit ([manuel officiel, alternative sous seuil](https://github.com/junegunn/fzf/blob/master/man/man1/fzf.1#L1106-L1113)). Il permet aussi de faire tourner l'aperçu entre plusieurs tailles et l'état masqué ([README officiel, preview window](https://github.com/junegunn/fzf#preview-window)).
- Bubbles fournit une vue d'aide courte/complète qui se génère depuis les keybindings et se tronque quand l'espace manque ; son viewport propose touches de pager et molette ([README officiel Bubbles](https://github.com/charmbracelet/bubbles#help)).
- btop offre des presets de disposition et un mode TTY limité à 16 couleurs et glyphes simples. Son propre changelog documente des correctifs de crash, de menu et de contraintes liés aux petits terminaux ([README officiel](https://github.com/aristocratos/btop#prerequisites), [changelog officiel](https://github.com/aristocratos/btop/blob/main/CHANGELOG.md)).
- Un utilisateur de lazygit rapporte que son mode « accordion » sur fenêtre étroite masque le contexte de branche et peut conduire à agir sur la mauvaise branche ([issue officielle #5377](https://github.com/jesseduffield/lazygit/issues/5377)). Cela montre un risque concret : compacter en cachant le contexte critique peut rendre l'interface dangereuse.

**Inférence Runny.** Politique de reflow recommandée :

| Taille | Disposition proposée | Éléments toujours visibles |
|---|---|---|
| `>= 100` colonnes | deux panneaux, environ 42/58 | commande, portée, workers, compteurs, focus |
| `80–99` colonnes | deux panneaux plus denses ; widgets décoratifs supprimés | même contexte critique |
| `60–79` colonnes | un panneau à la fois ; `tab` bascule Tasks/Output | bandeau `command · selected · mode · run status` |
| `< 60` colonnes ou très faible hauteur | vue mono compacte, aide réduite à `?`, lignes tronquées par largeur de cellules | commande tronquée mais identifiable, total sélectionné, erreurs |

À 80x24, le budget vertical doit prioriser : 2 lignes de contexte, saisie de commande, contenu, 1 ligne de message et 1–2 lignes d'aide. Les cadres imbriqués, logos et widgets récapitulatifs doivent disparaître avant le contenu. À 60 colonnes, ne pas comprimer deux panneaux jusqu'à les rendre inutilisables.

### 3. Sélection multiple : rendre portée et quantité explicites

- fzf utilise `Tab` et `Shift-Tab` pour marquer en mode multiple et affiche le nombre sélectionné, même quand il vaut zéro ([README officiel, Using the finder](https://github.com/junegunn/fzf#using-the-finder), [changelog officiel](https://github.com/junegunn/fzf/blob/master/CHANGELOG.md)).
- Le mode arbre de tmux distingue curseur et tags : `t` marque, `T` retire tous les tags, `X` agit sur les éléments marqués. Les éléments marqués portent un `*`, donc la sélection ne dépend pas seulement de la couleur ([guide officiel tmux](https://github.com/tmux/tmux/wiki/Getting-Started#choosing-sessions-windows-and-panes)).
- htop permet de taguer un groupe puis ouvre un menu de signal avant de tuer les processus ; sans tags, l'action vise la ligne courante ([manuel source officiel](https://github.com/htop-dev/htop/blob/main/htop.1.in)).

**Inférence Runny.** Chaque ligne devrait avoir trois canaux séparés :

- curseur : chevron `>` + inversion ou bordure ;
- sélection : case `[x]`, `[ ]` ou `[-]` pour un arbre partiel ;
- exécution : libellé et symbole, par exemple `RUN`, `OK`, `FAIL`, `QUEUE`, jamais couleur seule.

Le pied de liste doit afficher `12 visible · 5 selected · 2 running`. `a` devrait signifier sans ambiguïté « sélectionner tous les visibles » et `A` « désélectionner tous les visibles », plutôt qu'un toggle global dont l'effet varie selon l'état initial. Toute action doit nommer sa portée dans le message ou le dialogue : `Cancel 3 selected runs?`, jamais seulement `Cancel?`.

### 4. Logs parallèles et progression

- mprocs affiche la sortie de chaque commande séparément, permet de basculer entre processus, zoomer, régler le scrollback et entrer dans un mode de copie explicite ([README officiel](https://github.com/pvolok/dekit/blob/master/README-mprocs.md#config), [mode copie](https://github.com/pvolok/dekit/blob/master/README-mprocs.md#how-to-copy-text)).
- Overmind s'appuie sur le mode contrôle de tmux pour éviter que la sortie soit coupée ou retardée et pour préserver la couleur ; il permet aussi de se connecter à un processus précis ([README officiel](https://github.com/DarthSim/overmind#overmind-features)).
- fzf sait suivre une commande d'aperçu longue avec `follow`, afficher les résultats partiels et limiter explicitement l'historique initial afin de contenir la mémoire ([exemple officiel de log tailing](https://github.com/junegunn/fzf/blob/master/ADVANCED.md#log-tailing)).
- Bubbles fournit spinner, barre de progression et viewport, mais ces composants sont des primitives ; ils ne définissent pas à eux seuls une sémantique d'état ([README officiel Bubbles](https://github.com/charmbracelet/bubbles)).

**Inférence Runny.** Sortie doit rester par cible, pas devenir un flux fusionné illisible. Ajouter un mode `All events` peut aider au diagnostic, avec chaque ligne préfixée par cible et timestamp, mais la vue primaire doit suivre la cible au curseur. En-tête Output recommandé : `api [RUN] · follow:on · 184 lines · 00:12`. Un compteur global permanent doit permettre de comprendre l'avancement sans regarder les couleurs : `2 queued · 3 running · 6 ok · 1 failed`.

Le follow doit se désactiver automatiquement quand l'utilisateur remonte, afficher `follow:off`, puis se réactiver avec `f` ou `End`. La rétention en mémoire doit être bornée ; fichiers persistés peuvent conserver l'intégralité.

### 5. Raccourcis et découvrabilité

Les conventions les mieux partagées dans cet échantillon sont :

- `?` pour aide ; K9s affiche les mnémoniques actifs ([README officiel K9s](https://github.com/derailed/k9s#key-bindings)) et htop accepte `F1`, `h` ou `?` ([README officiel htop](https://github.com/htop-dev/htop#usage)) ;
- `/` pour recherche ou filtre dans fzf, lazygit, K9s et htop ;
- flèches comme chemin universel, avec `j/k` comme alias expert ;
- `Esc` pour revenir/fermer et `Enter` pour ouvrir/confirmer ;
- aide courte persistante, aide complète à la demande, keybindings configurables. Bubbles lie directement binding et texte d'aide pour éviter leur divergence ([README officiel Bubbles, Key](https://github.com/charmbracelet/bubbles#key)).

Limite observée : l'aide statique ne suffit pas. Un utilisateur de lazygit décrit un état de rebase où `?` liste des actions sans lui permettre de trouver « continuer » et propose une vue contextuelle fuzzy-searchable ([issue officielle #3141](https://github.com/jesseduffield/lazygit/issues/3141)). Autre limite : les combinaisons riches ne voyagent pas uniformément entre terminaux et multiplexeurs. La documentation tmux explique que les touches modifiées disponibles et leurs séquences varient selon le terminal ([guide officiel Modifier Keys](https://github.com/tmux/tmux/wiki/Modifier-Keys)), et lazygit avertit que ses nouveaux protocoles de clavier ne sont pas pris en charge partout ([release v0.62.0](https://github.com/jesseduffield/lazygit/releases/tag/v0.62.0)).

**Inférence Runny.** Garder comme primaire un noyau ASCII et touches standard. Éviter `Alt+Shift+...` comme seul accès à une fonction. Palette `:` recommandée : recherche fuzzy, catégories, keybinding affiché à droite, portée visible, marque `DANGER` pour actions destructives, commande exécutable directement. `?` devrait ouvrir la même base de commandes en mode documentation contextuelle afin d'éviter deux listes qui divergent.

Proposition de keymap :

| Contexte | Touches primaires | Décision |
|---|---|---|
| Global | `?`, `:`, `/`, `tab`, `shift-tab`, `z` | conserver conventions actuelles |
| Tasks | `↑/↓`, `j/k`, `space`, `a`, `A`, `enter` | déplacement, sélection, tous/aucun visible, lancer |
| Output | `pgup/pgdn`, `ctrl+u/d`, `g/G`, `f`, `v` | pager, début/fin, follow, copy mode |
| Annulation | `x` sélection/cible, `X` tous avec confirmation | séparer portée normale et globale |
| Quitter | `q` ou `ctrl+c`, confirmation si runs actifs | ne jamais surcharger `ctrl+c` comme copie selon un état caché |
| Copier | `v` puis `y`, ou commande palette | ne pas dépendre de raccourcis clavier étendus |

### 6. Souris : amélioration secondaire, coût réel

- btop annonce un support souris complet, avec boutons signalés comme cliquables et molette dans listes/menus ([README officiel](https://github.com/aristocratos/btop#features)). K9s expose `enableMouse` comme préférence configurable ([documentation officielle des skins/config](https://k9scli.io/topics/skins/)). htop propose explicitement `--no-mouse` ([manuel source officiel](https://github.com/htop-dev/htop/blob/main/htop.1.in)).
- Bubble Tea indique que son mode cellule reçoit clic, relâchement et molette ([source officielle Bubble Tea v2](https://github.com/charmbracelet/bubbletea/blob/main/tea.go)). Une issue toujours ouverte rapporte que l'activation de la souris permet le scroll applicatif mais empêche la sélection native du texte ([issue officielle #162](https://github.com/charmbracelet/bubbletea/issues/162)).
- Les hitboxes dérivées du rendu sont fragiles : btop a reçu un rapport où le décalage augmente horizontalement jusqu'à exiger un clic hors du menu ([issue officielle #998](https://github.com/aristocratos/btop/issues/998)).

**Inférence Runny.** Souris doit dupliquer clavier, jamais être obligatoire. Fournir `mouse: true|false` ou `--no-mouse`. Clic sur panneau donne focus, clic ligne déplace curseur, clic case change sélection, molette agit sous pointeur. Ne pas rendre toute ligne sélectionnante si l'utilisateur peut vouloir copier du texte. Ajouter tests de hitbox après resize, Unicode, reflow mono et zoom. Un mode copie explicite réduit le conflit, sans éliminer besoin d'un opt-out souris.

### 7. Couleur et accessibilité

- WCAG 2.2 exige une alternative visible à la couleur pour transmettre information ou action ([W3C, Use of Color](https://www.w3.org/WAI/WCAG22/Understanding/use-of-color)) et recommande au moins 4,5:1 pour texte normal ([W3C, Contrast Minimum](https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html)). WCAG vise le Web, mais ces principes perceptifs restent une base utile pour une TUI ; leur application à Runny est une extrapolation.
- Une issue K9s explique qu'une colonne active signalée seulement par couleur et gras reste difficile à identifier avec déficience de vision des couleurs ou skin peu contrasté ; elle propose l'inversion, signal terminal indépendant de la teinte ([issue officielle #3955](https://github.com/derailed/k9s/issues/3955)).
- Lip Gloss sait réduire automatiquement le profil colorimétrique, jusqu'au profil ASCII noir et blanc, et recommande de demander la couleur de fond dans Bubble Tea pour choisir une variante claire/sombre ([documentation officielle](https://github.com/charmbracelet/lipgloss#colors), [adaptive colors](https://github.com/charmbracelet/lipgloss#adaptive-colors)).
- La convention `NO_COLOR` demande que toute variable non vide désactive les couleurs ANSI ajoutées par défaut ([spécification de fait NO_COLOR](https://no-color.org/)). htop fournit `--no-color` et `--no-unicode`; btop fournit `--low-color` et `--tty` ([manuel htop](https://github.com/htop-dev/htop/blob/main/htop.1.in), [README btop](https://github.com/aristocratos/btop#command-line-options)).
- btop documente aussi les problèmes de largeur et d'alignement causés par certains glyphes Unicode, polices ou terminaux web ([README officiel, prerequisites](https://github.com/aristocratos/btop#prerequisites)).

**Inférence Runny.** Ajouter ou garantir `--color=auto|always|never`, respecter `NO_COLOR`, puis tester capture réelle en mode mono. Ne pas forcer TrueColor en mode `auto`. Associer couleur, symbole et texte à chaque état. Prévoir thème clair/sombre ou couleurs ANSI sémantiques, plus mode ASCII sans icônes ambiguës. Vérifier contrastes sur fond clair, fond sombre, 256 couleurs et mono. Mesurer largeur par cellule/grapheme, jamais par octets ou runes isolées.

### 8. Alternate screen et hygiène terminal

- Bubble Tea v2 porte `AltScreen`, `MouseMode`, focus reporting, curseur et bracketed paste dans la vue. Sa documentation indique que l'alternate screen est quitté automatiquement à l'arrêt ; sa chaîne de shutdown restaure l'état terminal ([source officielle `tea.go`](https://github.com/charmbracelet/bubbletea/blob/main/tea.go), [guide de migration v2](https://github.com/charmbracelet/bubbletea/blob/main/UPGRADE_GUIDE_V2.md#the-big-idea-declarative-views)).
- Cette garantie de framework ne supprime pas tous les cas limites. Bubble Tea conserve des rapports ouverts concernant perte d'entrée lors de `ReleaseTerminal()`/shutdown ([issue #616](https://github.com/charmbracelet/bubbletea/issues/616)), artefacts de resize en alternate screen ([issue #573](https://github.com/charmbracelet/bubbletea/issues/573)) et non-restauration de la souris après `ExecProcess` ([issue #1424](https://github.com/charmbracelet/bubbletea/issues/1424)). Lazygit a aussi documenté un problème complexe de terminal de contrôle autour de sous-processus interactifs ([issue #4320](https://github.com/jesseduffield/lazygit/issues/4320)).

**Inférence Runny.** Garder alternate screen convient au dashboard. Tests PTY doivent couvrir sortie normale, `q`, `ctrl+c`, panic contrôlée, resize répété et lancement de sous-processus. Après sortie, curseur, echo, souris, bracketed paste et buffer principal doivent être restaurés. Messages d'erreur fatale doivent être imprimés après restauration, pas enfermés dans écran alternatif disparu.

### 9. Erreurs et actions destructives

- mprocs sépare arrêt doux `x`, arrêt forcé `X`, quit doux `q` et force quit `Q`; son contrôle distant possède aussi `quit-or-ask` distinct de `quit` ([README officiel](https://github.com/pvolok/dekit/blob/master/README-mprocs.md#default-keymap), [remote control](https://github.com/pvolok/dekit/blob/master/README-mprocs.md#remote-control)).
- K9s permet à un plugin d'afficher commande exacte et confirmation avant exécution ; sa description est montrée près du raccourci ([documentation officielle Plugins](https://github.com/derailed/k9s#plugins)). Lazygit conserve des warnings configurables pour amend, discard, stash et autres opérations sensibles ([configuration officielle](https://github.com/jesseduffield/lazygit/blob/master/docs/Config.md#default)).
- Une petite largeur peut devenir un défaut de sûreté, pas seulement esthétique : rapport btop d'un crash sur fenêtre étroite ([issue officielle #874](https://github.com/aristocratos/btop/issues/874)) et rapport lazygit de contexte critique caché en accordéon ([issue officielle #5377](https://github.com/jesseduffield/lazygit/issues/5377)).

**Inférence Runny.** Avant lancement, rendre visibles commande, nombre de cibles, mode serial/parallel et workers. Confirmation recommandée quand portée dépasse une cible, quand commande vient d'être modifiée, ou quand préférence de sécurité l'impose. Dialogue doit montrer une prévisualisation des cibles, pas seulement `Run?`. Annulation globale et force quit exigent confirmation. Erreur doit rester jusqu'à action explicite, nommer cible et cause, proposer prochaine action (`retry`, `open log`, `copy error`) et ne jamais être remplacée immédiatement par une notification de moindre priorité.

## Refonte graphique proposée

### Écran large

```text
runny  command: pnpm test                  parallel · 4 workers · 5 selected
┌ TASKS — FOCUSED ───────────┬ OUTPUT: api [RUN] · follow:on · 184 lines ┐
│ > [x] RUN   api       00:12 │ ...                                     │
│   [x] OK    web       00:08 │ PASS src/routes.test.ts                 │
│   [ ] IDLE  docs            │ ...                                     │
│   [x] FAIL  worker    00:03 │ Error: connection refused               │
└─────────────────────────────┴─────────────────────────────────────────┘
2 queued · 1 running · 1 ok · 1 failed     space select  / filter  ? help
```

Principes : une seule bordure structurante, pas de cartes décoratives ; états textuels alignés ; commande et portée en tête ; résumé persistant en bas ; aide strictement contextuelle.

### Écran 60 colonnes

```text
runny · pnpm test · 5 selected · parallel/4
TASKS [focused]                         tab: Output
> [x] RUN   api                         00:12
  [x] OK    web                         00:08
  [ ] IDLE  docs
  [x] FAIL  worker                      00:03
...
1 run · 1 ok · 1 fail       space select · ? help
```

`tab` passe à Output sans perdre commande, sélection et résumé. `z` maximise puis revient. L'aide complète devient overlay paginé ; le footer ne tente pas d'afficher tous les raccourcis.

## Priorités proposées

### P0 — sûreté et compréhension

1. Séparer visuellement curseur, sélection et état ; ajouter texte/symboles indépendants de couleur.
2. Rendre commande, portée, workers et compteurs permanents dans tous les layouts.
3. Passer en mono-panneau sous environ 80 colonnes ; ne jamais masquer contexte critique.
4. Clarifier annulations : `x` portée sélectionnée/courante, `X` tout avec confirmation ; supprimer surcharge cachée de `ctrl+c` comme copie.
5. Garantir et tester `NO_COLOR`, faible couleur, fond clair/sombre, ASCII et restauration terminal.

### P1 — efficacité

1. Palette fuzzy unifiée avec aide, keybindings, portée et danger.
2. Follow explicite, auto-pause au scroll, `f`/`End` pour reprendre.
3. Mode copie clavier, scrollback borné et ouverture directe du log complet.
4. Aide contextuelle courte dérivée de la même définition que la keymap.
5. Remappage des keybindings avec détection de conflits.

### P2 — confort

1. Opt-out souris et hitboxes testées sur resize/reflow/Unicode.
2. Vue agrégée `All events` préfixée par cible.
3. Accès direct aux premières cibles si cela reste compatible terminal.
4. Thèmes, densité et ratio de panneaux configurables après stabilisation du reflow.

## Matrice de validation recommandée

| Axe | Cas minimum |
|---|---|
| Dimensions | 120x40, 100x30, 80x24, 60x24, 50x16, resize pendant run |
| Terminal | Ghostty direct, tmux, terminal 256 couleurs, `TERM=dumb` ou mode dégradé pertinent |
| Couleur | fond clair/sombre, `NO_COLOR=1`, faible couleur, contraste des focus/erreurs |
| Texte | ASCII, accents, CJK, emoji, chemins longs, commande multi-ligne |
| Entrées | clavier seul, souris off/on, sélection native, scroll, copy mode |
| Exécution | zéro/une/beaucoup de cibles, serial/parallel, logs rapides, logs immenses |
| États | queued/running/success/failure/cancelled/skipped, mélange simultané |
| Sûreté | annulation cible/sélection/tout, quit avec runs actifs, force quit |
| Hygiène | sortie normale, `ctrl+c`, panic, sous-processus, curseur/echo/mouse restaurés |

## Limites

- Aucun des projets étudiés ne publie, dans les sources consultées, une étude contrôlée comparant ses layouts ou raccourcis. Les « succès » sont donc des patterns maintenus dans des outils largement adoptés, pas une causalité démontrée.
- Issues GitHub sélectionnées sont des signaux qualitatifs et parfois dépendants d'un terminal, d'une police ou d'une ancienne version.
- Cette note ne remplace pas tests utilisateurs de Runny. Idéal : 5–8 personnes effectuent sélection filtrée, lancement, identification d'un échec, lecture/copie du log et annulation, à 80x24 puis 60x24 ; mesurer erreurs, hésitations et recours à `?`.
- WCAG concerne contenu Web. Principes de couleur, contraste et focus sont transposés ici comme heuristiques perceptives, pas comme certification de conformité d'une TUI.
