# Optimiser le scroll d'un Output long

**Recommandation : conserver Bubbles, préparer les lignes visuelles lorsque le contenu ou la largeur change, puis limiter le travail du scroll à la fenêtre visible.** Les exemples Textual et ov étayent cette séparation ; la source Bubbles montre pourquoi le cache de contenu seul ne suffit pas avec son wrapping actuel. La section finale détaille l'application proposée et ses limites.

Recherche du 10 septembre 2026. Sources primaires : documentation officielle et code des projets. Les liens vers le code sont figés sur les commits consultés ; ils ne constituent pas une recommandation de version. Context7 a servi à retrouver la documentation Textual, ensuite vérifiée à sa source.

Cette recherche compare des mécanismes, pas des performances mesurées entre frameworks. Aucun benchmark Textual/Ratatui/ov n'a été exécuté. Les complexités indiquées sont des déductions du code, à valider dans Runny.

## Ce que font les autres TUI

### Textual : produire les lignes demandées par le viewport

La documentation distingue le rendu d'un widget entier et la **Line API**. Avec `ScrollView`, l'application expose une `virtual_size`, puis `render_line(y)` fabrique une ligne de la zone visible en tenant compte du défilement. Textual présente cette API pour les widgets volumineux ou fréquemment mis à jour. Le travail de rendu peut donc porter sur les lignes affichées, même si le document logique est beaucoup plus grand. [Documentation officielle, Line API et Scrolling](https://textual.textualize.io/guide/widgets/#line-api).

Deux widgets illustrent des compromis différents :

- **Log** conserve des lignes brutes ; `render_line` accède à `scroll_y + y`, puis découpe horizontalement le résultat. `write_lines` ajoute un lot et confie le calcul de sa largeur maximale à un worker ; la largeur virtuelle est ensuite mise à jour. Le calcul porte sur les nouvelles lignes. Limite : ce widget utilise `no_wrap=True`, donc son accès direct ne résout pas le défilement de lignes repliées. [Code Log, commit `06dbeef`](https://github.com/Textualize/textual/blob/06dbeef4bb70fb718236aa418ed658ef4667a126/src/textual/widgets/_log.py#L130-L141), [ajout et rendu](https://github.com/Textualize/textual/blob/06dbeef4bb70fb718236aa418ed658ef4667a126/src/textual/widgets/_log.py#L215-L351).
- **RichLog** calcule le rendu et les lignes éventuellement repliées lors de `write`, puis conserve des `Strip`. Le scroll indexe directement ces lignes et utilise un cache LRU de 1 024 découpes, avec une clé incluant l'identité de ligne, l'offset horizontal et la largeur. `max_lines` permet de borner l'historique. Limites : un gros `write` effectue son travail à l'insertion ; le gestionnaire de resize débloque les écritures initialement différées, mais ne recalcule pas le wrapping de l'historique existant. C'est donc un exemple de préparation hors du chemin de scroll, pas un modèle complet pour un historique qui doit se replier à chaque redimensionnement. [Code RichLog, initialisation et resize](https://github.com/Textualize/textual/blob/06dbeef4bb70fb718236aa418ed658ef4667a126/src/textual/widgets/_rich_log.py#L99-L139), [write et render_line](https://github.com/Textualize/textual/blob/06dbeef4bb70fb718236aa418ed658ef4667a126/src/textual/widgets/_rich_log.py#L175-L320).

La leçon transférable est de séparer **contenu**, **mise en lignes** et **fenêtre visible**. Un déplacement du viewport ne devrait pas être assimilé à une nouvelle écriture du document. C'est une proposition de conception déduite de ces exemples, pas une garantie fournie automatiquement par toute bibliothèque TUI.

### Ratatui : le rendu différentiel ne remplace pas un index de wrapping

Ratatui reconstruit le contenu du buffer de terminal à chaque frame demandée par l'application. Il compare ensuite le buffer courant au précédent et n'envoie au terminal que les cellules modifiées. Cela réduit les écritures terminal ; cela ne supprime pas les calculs effectués auparavant par les widgets. [Modèle de rendu](https://ratatui.rs/concepts/rendering/), [buffers et flush](https://ratatui.rs/concepts/rendering/under-the-hood/#flush).

Le code actuel de `Paragraph` montre précisément cette distinction. Sans wrapping, il saute aux lignes correspondant à l'offset avant de les rendre. Avec wrapping, il crée un `WordWrapper` et appelle `next_line()` pour chaque ligne visuelle précédant l'offset, puis dessine seulement la hauteur disponible. Le commentaire du code décrit explicitement ce parcours. **Un Paragraph replié placé très loin dans le texte conserve donc un coût de parcours du préfixe à chaque rendu** ; le clipping final ne fournit pas un accès indexé. Cette conclusion porte sur ce widget et ce commit, pas sur tous les widgets de l'écosystème Ratatui. [Paragraph, commit `982cd76`, lignes 421–460](https://github.com/ratatui/ratatui/blob/982cd76bcabf3899bd35e7ad10f16553e95c84db/ratatui-widgets/src/paragraph.rs#L421-L460).

Changer de framework ne garantit donc pas de résoudre le problème de Runny. Il faut vérifier où se trouvent le découpage, la mesure Unicode et l'accès à l'offset, même si le terminal reçoit déjà un diff minimal.

### ov : un pager Go qui conserve une position dans le document

Le pager `ov` représente le haut de page par `topLN` et `topLX` : ligne logique et position à l'intérieur de cette ligne. `drawBody` part de cette position et avance jusqu'à la limite verticale de l'écran. Le déplacement peut ainsi être local au document, sans reconstruire toutes les lignes précédentes pour atteindre une position globale. [Déplacements, commit `25207cb`](https://github.com/noborus/ov/blob/25207cb07f33456f38b5435266c238e6ac238821/oviewer/move_updown.go#L12-L67), [boucle de dessin](https://github.com/noborus/ov/blob/25207cb07f33456f38b5435266c238e6ac238821/oviewer/draw.go#L46-L93).

La préparation du corps couvre au plus une hauteur d'écran de lignes logiques à partir du haut courant, en plus des en-têtes. Ces lignes passent par `getLineC`, qui conserve leur résultat d'analyse dans un cache LRU limité à 10 000 entrées ; un hit copie les cellules déjà analysées avant application des styles. Il reste donc du travail par ligne visible et par longueur de ligne, mais pas une analyse intégrale systématique de l'historique pour préparer le corps de page. [Plage préparée](https://github.com/noborus/ov/blob/25207cb07f33456f38b5435266c238e6ac238821/oviewer/prepare_draw.go#L155-L159), [préparation et styles](https://github.com/noborus/ov/blob/25207cb07f33456f38b5435266c238e6ac238821/oviewer/prepare_draw.go#L411-L446), [cache et analyse](https://github.com/noborus/ov/blob/25207cb07f33456f38b5435266c238e6ac238821/oviewer/document.go#L608-L646), [capacité du cache](https://github.com/noborus/ov/blob/25207cb07f33456f38b5435266c238e6ac238821/oviewer/document.go#L32-L33).

Pour la mémoire, `ov` documente également une lecture par blocs pour les gros fichiers et des limites de blocs conservés. Les fichiers seekables permettent de relire les blocs évincés ; les flux non seekables demandent une politique différente. Cette optimisation de stockage est distincte de celle du scroll. [README officiel, réduction de mémoire](https://github.com/noborus/ov#5-how-to-reduce-memory-usage).

## Limites communes à garder explicites

Un cache échange du CPU contre de la mémoire et exige une invalidation correcte. Un rendu limité au visible peut encore être cher sur une unique ligne immense. Un redimensionnement peut nécessiter une nouvelle mise en lignes, même si le scroll à largeur constante est rapide. Ces conséquences découlent des unités de travail observées ci-dessus ; aucun des exemples ne justifie une promesse de latence sans mesure sur le contenu réel.

Pour un flux actif, regrouper les ajouts et dissocier leur fréquence du rafraîchissement sont des pistes supplémentaires. L'exemple vérifié ici est `Log.write_lines`, qui traite un lot et mesure les nouvelles lignes. Cela ne prouve ni une cadence universelle optimale, ni qu'une limitation des FPS résoudrait le coût d'un cran sur un Output déjà terminé.

## Bubbles : options vérifiées pour Runny

Runny utilise Bubbles **v2.2.1**, dernière version stable publiée au moment de la recherche, le 24 août 2026. Le fichier `viewport.go` de cette version, celui du module local et celui de `main` au commit `0a69b19b0690e9504a511fc231f69cea59ba1cc6` ont le même SHA-256 (`6d5d15d15d175464237df578efd2f9f3022e89874649454ee69c30b1a250dec7`). Une montée de version ne fournit donc pas de correction de ce chemin à cette date. [Release officielle](https://github.com/charmbracelet/bubbles/releases/tag/v2.2.1), [source main figée](https://github.com/charmbracelet/bubbles/blob/0a69b19b0690e9504a511fc231f69cea59ba1cc6/viewport/viewport.go).

Trois points ressortent du code :

- `SetContentLines` mesure toutes les lignes ; `calculateLine`, lorsque `SoftWrap` est actif, les parcourt à nouveau pour déterminer les positions. Sans wrapping, cette dernière fonction obtient directement le nombre et l’index des lignes.
- `StyleLineFunc` existe déjà. `visibleLines` sélectionne une tranche, puis `styleLines` applique les styles à cette tranche. C’est une alternative directe à la coloration de tout l’Output par Runny.
- Garder le contenu préparé évite les rechargements, mais ne supprime pas le parcours global de `calculateLine` avec `SoftWrap=true`. [Source Bubbles v2.2.1, méthodes concernées](https://github.com/charmbracelet/bubbles/blob/v2.2.1/viewport/viewport.go#L233-L378).

La PR upstream #823 a amélioré allocations et CPU, mais elle a été fusionnée en septembre 2025 et son travail est déjà présent dans v2.2.1. Ses benchmarks ne constituent pas une mesure du gain encore disponible dans Runny. [PR #823 et benchmarks](https://github.com/charmbracelet/bubbles/pull/823).

L’ancien `HighPerformanceRendering` a été supprimé de Bubbles v2. Bubble Tea v2 délègue le rendu terminal à Ultraviolet, qui compare les cellules et optimise leur émission. Cette couche intervient après la préparation du texte : elle ne retire pas le travail global effectué dans `Update`/`View`. [Migration Bubbles v2](https://github.com/charmbracelet/bubbles/blob/v2.2.1/UPGRADE_GUIDE_V2.md), [renderer Bubble Tea v2.0.8](https://github.com/charmbracelet/bubbletea/blob/v2.0.8/cursed_renderer.go#L257-L462), [renderer Ultraviolet utilisé par Runny](https://github.com/charmbracelet/ultraviolet/blob/f5a850f9c2b7ed4def3807fe12d0b5e76d345bdc/terminal_renderer.go).

## Exemple supplémentaire : Glow, dans le même écosystème Go

Glow sépare préparation Markdown et navigation : le résultat arrive par `contentRenderedMsg`, qui met à jour le viewport. Le redimensionnement relance `renderWithGlamour` ; `View` réutilise ensuite le viewport. Ce code illustre où placer le travail coûteux, sans démontrer que tous ses cas de scroll ont un coût constant. [Glow, commit 7b2431d](https://github.com/charmbracelet/glow/blob/7b2431d4a82428fb477eb4361e11583e1644e9ba/ui/pager.go#L182-L218), [préparation asynchrone](https://github.com/charmbracelet/glow/blob/7b2431d4a82428fb477eb4361e11583e1644e9ba/ui/pager.go#L342-L350).

## Application proposée à Runny

Ce qui suit est une recommandation issue des sources et du diagnostic local, pas une optimisation déjà implémentée ou un gain mesuré.

Le diagnostic précédent, sur la révision `93ad57b6e53634eeb1d1f58fa66f56820a9516cf`, mesure environ **270 ms par événement molette + View** pour 50 000 lignes ASCII (3,25 Mo), fenêtre 120 × 32, sortie figée. Le profiler situe l’essentiel du CPU dans le calcul des largeurs. La désactivation du GC n’améliore pas cette fixture. Ces chiffres mesurent Runny, pas Textual, Ratatui ou ov. [Diagnostic local et méthode](/Users/saewyn/.codex/visualizations/2026/09/10/01a08d34-caed-7772-ad2b-6f22389ca601/scroll-diagnosis/DIAGNOSTIC.md).

### Deux niveaux de correction

| Option | Action | Limite |
|---|---|---|
| Réduction immédiate du travail | Supprimer la double synchronisation molette ; conserver le contenu ; appliquer les styles via `StyleLineFunc`. | `SoftWrap` continue de parcourir le buffer à chaque scroll. |
| Recommandée | Préparer un index/cache de lignes visuelles lorsque contenu ou largeur change ; utiliser un viewport sans wrapping sur des lignes **déjà découpées**. | Construction initiale, redimensionnement et invalidation restent à optimiser et mesurer. |
| Évolution upstream | Ajouter à Bubbles un index de hauteurs/positions et des caches invalidés correctement. | Patch de bibliothèque et maintenance plus larges ; ne pas commencer par un fork pour ce seul besoin. |

Dans l’option recommandée, le comportement reste un affichage avec retours visuels à la ligne. `SoftWrap=false` désactive seulement leur recalcul dans Bubbles ; le découpage est réalisé en amont. Il ne faut pas fournir des lignes brutes trop longues à ce viewport.

Pour un cache chaud, l’objectif algorithmique est un déplacement d’offset en O(1), puis un travail proportionnel aux cellules visibles. Avec un index hiérarchique plutôt qu’un tableau de lignes visuelles, la recherche peut être O(log L). Ce sont des objectifs de conception, pas des garanties de latence. Une reconstruction intégrale demeure O(B) en octets conservés. Les notifications de sortie inchangée, les ticks et le scroll ne doivent pas la déclencher.

### Détails nécessaires pour conserver le comportement

- **Clé et invalidation.** Associer la préparation à la Target, au contenu effectif et à la largeur utile ; intégrer les paramètres de rendu pertinents. Une modification de hauteur seule ne nécessite pas de redécoupage. Traiter explicitement nouveau Run, changement de Target, remplacement du suffixe retenu et marqueur de troncature. Les snapshots sont appliqués dans [model.go](/Users/saewyn/.codex/worktrees/25bf/runny/internal/tui/model.go:1151).
- **Texte et index.** Conserver séparément texte original, frontières de lignes logiques, fragments visuels et offsets de sélection. Le niveau d’erreur/couleur d’un fragment doit venir de sa ligne logique, même si le mot `error` apparaît dans un autre fragment. Le mode sélection possède déjà un index par largeur et un rendu limité aux lignes visibles ; il utilise toutefois un snapshot normalisé sans ANSI et ne peut pas remplacer directement le rendu normal. [output_selection.go](/Users/saewyn/.codex/worktrees/25bf/runny/internal/tui/output_selection.go:54), [rendu sélection](/Users/saewyn/.codex/worktrees/25bf/runny/internal/tui/output_selection.go:231), [normalisation](/Users/saewyn/.codex/worktrees/25bf/runny/internal/tui/output_text.go:12).
- **ANSI et Unicode.** `ansi.Hardwrap` prend en compte les graphèmes, les caractères larges et conserve les séquences ANSI ; `preserveSpace=true` préserve les espaces en début de ligne. C’est une primitive à évaluer, pas une intégration prête à l’emploi : après découpage en fragments affichables indépendamment, il faut restaurer l’état ANSI actif au début du fragment et vérifier les offsets de copie. [Source Hardwrap v0.11.7](https://github.com/charmbracelet/x/blob/ansi/v0.11.7/ansi/wrap.go#L14-L86).
- **Longue ligne unique.** Préparer ses frontières en une passe ; éviter de rescanner depuis son début pour chaque fragment. Sinon le scroll peut être rapide sur 50 000 petites lignes et rester coûteux sur quelques mégaoctets sans retour à la ligne.
- **Sortie active.** Commencer par le cache de la Target visible, avec mémoire bornée. Pour l’ajout incrémental, retraiter la dernière ligne incomplète puis les nouvelles données ; une éviction du préfixe doit ajuster index et ancrage. Runny regroupe déjà les notifications de sortie non consommées par Target : ne pas ajouter un mécanisme équivalent comme remède principal au scroll figé. [event_queue.go](/Users/saewyn/.codex/worktrees/25bf/runny/internal/run/event_queue.go:34), [rétention](/Users/saewyn/.codex/worktrees/25bf/runny/internal/run/tail.go:3).
- **Travail en arrière-plan.** À envisager si la préparation initiale ou le resize bloque encore : préparer depuis un snapshot immuable, puis appliquer uniquement un résultat correspondant à la Target, à la révision et à la largeur actuelles. Cela prévient un ancien calcul écrasant un résultat récent. Lancer une goroutine par cran ne traite pas la cause.

### Validation d’une future correction

Mesurer séparément : scroll sur cache chaud, première ouverture, resize, nouvelles sorties et éviction du buffer. Reprendre la boucle réelle `Update(MouseWheelMsg)+View` avec 100, 10 000 et 50 000 lignes, puis ANSI, Unicode large/combinant, ligne unique très longue et fenêtre étroite. Comparer médiane, p95, octets alloués et croissance du coût avec le volume, dans les mêmes conditions.

Ajouter des tests fonctionnels pour le wrapping, les bornes de scroll, follow, changement de Target, sélection/copie et troncature, puis inspecter dans Ghostty. Pour le cache chaud, un budget de quelques millisecondes par interaction peut servir d’objectif local ; aucun facteur d’accélération précis n’est établi par cette recherche.

**Décision proposée : garder Bubble Tea/Bubbles et déplacer préparation, indexation et styles hors du chemin de scroll.** Aucun changement de comportement ou de dépendance n’est nécessaire pour commencer cette optimisation.

## Mise en œuvre et résultats locaux

Correctif appliqué après validation de cette proposition. Output et History conservent chacun le dernier découpage par contenu et largeur, dans `logLayoutCache`. Bubbles reçoit des lignes déjà découpées avec `SoftWrap=false` ; les styles de l’Output sont appliqués aux lignes visibles. La molette ne synchronise plus deux fois le contenu. Le compteur de lignes du titre réutilise aussi le cache.

Le découpage parcourt chaque ligne une seule fois, restaure SGR et hyperliens à chaque fragment et préserve leur état entre lignes logiques. La sélection conserve les offsets dans le texte original ; son calcul des tabulations correspond désormais aux quatre cellules utilisées pour l’affichage. Ultraviolet, déjà présent dans les dépendances transitives, devient une dépendance directe à la même version pour interpréter cet état ANSI.

Mesures sur Apple M4, fixture ASCII et chaîne réelle `Update(MouseWheelMsg)+View`, sans mesure du transport terminal :

| Mesure | Avant | Après |
|---|---:|---:|
| Repro initiale, 50 000 lignes / 3,25 Mo | 268–272 ms/cran | 0,233 ms/cran |
| Benchmark, 50 000 lignes | 325 ms/op | environ 0,22–0,23 ms/op |
| Allocations, 50 000 lignes | 101 482/op | 1 430/op |
| Octets alloués, 50 000 lignes | environ 8,15 Mo/op | environ 60 Ko/op |

Le benchmark après correction reste autour de 0,22 ms entre 100, 10 000 et 50 000 lignes. La préparation initiale mesurée séparément coûte environ 26–41 ms selon la fixture (ligne unique, lignes courtes ou ANSI). Elle reste synchrone et intégrale lors d’un changement de contenu ou de largeur ; l’ingestion incrémentale et la préparation en arrière-plan ne font pas partie de ce correctif.

Commandes de validation :

```sh
rtk proxy go test ./internal/tui -run '^$' -bench '^BenchmarkOutputScroll$' -benchtime=100x -count=3
rtk proxy go test ./internal/tui -run '^$' -bench '^BenchmarkOutputLayout$' -benchtime=3x
rtk go test ./...
rtk proxy go vet ./internal/tui
rtk go build ./cmd/runny
```

Le garde-fou automatisé vérifie que les allocations du chemin complet de scroll ne croissent pas avec les lignes retenues ; il était rouge avant le correctif. Tests fonctionnels ajoutés : invalidation, snapshots de viewport, ANSI/Unicode/hyperliens, couleur d’erreur par ligne logique, correspondance de sélection et tabulations. Ces tests et les tests existants ciblés sur scroll, copie et History passent aussi avec `-race`.

Suite complète : **454 réussites, 3 échecs préexistants**, identiques au diagnostic sur le code inchangé : `TestMouseClickMovesCursorToVisibleDirectoryRow`, `TestTargetRowsHighlightSelectedAndPartialSubtrees`, `TestTargetRowsKeepSelectionHighlightUnderFocus`. Build et `go vet` réussissent.

Inspection Ghostty : sortie de 50 001 lignes, largeurs normale et étroite, follow et navigation clavier ; aucun fond explicite dans les cellules inspectées hors sélection. L’injection de molette fournie par l’automatisation Ghostty n’a pas donné de déplacement observable ; la validation de la molette repose sur les événements `tea.MouseWheelMsg` du test et du benchmark. [Capture normale](/Users/saewyn/.codex/visualizations/2026/09/10/01a08d34-caed-7772-ad2b-6f22389ca601/scroll-fixed-wide.png), [capture étroite](/Users/saewyn/.codex/visualizations/2026/09/10/01a08d34-caed-7772-ad2b-6f22389ca601/scroll-fixed-narrow.png).
