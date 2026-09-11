# Vue Terraform globale : inspirations TUI

Recherche primaire consultée le 11 septembre 2026. Périmètre : quatre projets. Les comportements décrits dans « Observé » proviennent de leur documentation officielle ; les adaptations et évaluations qui suivent sont nos propositions pour Runny. Les dimensions ci-dessous sont des estimations de conception, pas des tests exécutés dans ces autres applications.

## Ce que font réellement les autres TUI

**Lazygit — des annotations au plus près de l’objet.** Observé : la configuration prévoit une arborescence de fichiers (`showFileTree`) ainsi que des nombres de lignes ajoutées/supprimées dans la vue Files (`showNumstatInFilesView`). Elle définit également la largeur du panneau latéral et peut accroître la hauteur du panneau ayant le focus. C’est une référence particulièrement directe pour enrichir une liste existante avec une information quantitative par objet. Source : [configuration officielle de Lazygit](https://github.com/jesseduffield/lazygit/blob/master/docs/Config.md).

Adaptation : conserver l’arbre Tasks et ajouter `+3 ~2 -1` sur chaque Target terminé. Ce placement permet de retrouver immédiatement environnement, chemin et résultat. La contrepartie vient de la largeur : indentation, nom, statut, durée et trois valeurs se disputent les mêmes cellules. Le modèle est excellent pour se repérer ; il est moins efficace pour comparer cent résultats numériques.

**K9s — une table dont les colonnes ont un rôle explicite.** Observé : les colonnes de ses vues de ressources peuvent être choisies et réordonnées ; les attributs distinguent nombres, alignement à droite, colonnes toujours visibles ou réservées au mode large. Une colonne de tri par défaut peut être définie. Source : [Custom Views de K9s](https://k9scli.io/topics/columns/).

Adaptation : une vue globale pleine largeur `STACK | STATUS | + | ~ | - | TIME`, avec nombres alignés à droite. Le nom et les trois compteurs sont prioritaires ; les détails secondaires s’ajoutent lorsque la largeur le permet. C’est la meilleure référence pour parcourir verticalement les suppressions ou comparer les stacks sans sélectionner chacune. Elle remplace cependant la proximité permanente entre Tasks et Output par un basculement explicite vers le détail.

**bottom — plusieurs organisations des mêmes objets.** Observé : son tableau de processus propose recherche, tri, regroupement par nom et arbre parent/enfant repliable. La documentation précise que regroupement et arbre sont incompatibles ; en mode regroupé, une colonne donne le nombre d’entrées et des consommations sont additionnées. Source : [Process Widget de bottom, documentation stable](https://bottom.pages.dev/stable/usage/widgets/process/). Un widget peut occuper tout le terminal ; le mode Basic retire les graphiques et n’affiche qu’un tableau à la fois. Sources : [agrandissement d’une vue](https://bottom.pages.dev/nightly/usage/general-usage/#expansion), [Basic Mode](https://bottom.pages.dev/nightly/usage/basic-mode/).

Adaptation : proposer une organisation par impact distincte de l’organisation par dossier. bottom fournit le principe du regroupement, **pas** un tableau Terraform classé par risque. Le classement « suppressions / autres changements / inchangées / en attente / erreurs » serait notre invention. Les en-têtes compteraient des stacks ; chaque ligne conserverait ses propres compteurs. Aucun besoin d’additionner les ressources de toutes les stacks.

**oxker — une vue d’ensemble libérée des logs à la demande.** Observé : la liste de conteneurs se trie par ses en-têtes, se filtre, et possède une navigation par panneau. Le panneau de logs peut être redimensionné en hauteur ou masqué. Source : [contrôles officiels d’oxker](https://github.com/mrjackwills/oxker#run).

Adaptation : rendre Output facultatif pendant la lecture du bilan global. Le mécanisme serait à concevoir avec le contrat de focus de Runny ; les raccourcis d’oxker ne constituent pas une recommandation de nouvelles touches.

## Trois alternatives à éprouver

| Proposition | 60 colonnes | 80 colonnes | 120 colonnes | 10 puis 100 stacks |
|---|---|---|---|---|
| **Arbre annoté** | Une vue Tasks, noms abrégés ; compteurs compacts prioritaires | Arbre confortable en pleine largeur | Le split existant limite encore Tasks à environ 50 colonnes ; annotations serrées | Excellent repérage à 10 ; à 100, branches et profondeur augmentent le défilement |
| **Table pleine largeur** | Chemin abrégé, statut court, trois valeurs ; durée secondaire | Six colonnes lisibles pour chemins raisonnables | Chemins longs et durée trouvent leur place | Lecture immédiate à 10 ; meilleure densité à 100, tri et filtre utiles |
| **Liste regroupée par impact** | Sections empilées, une ligne par stack | Même organisation, noms plus complets | Sections pleine largeur, ou grille expérimentale | À 10, toutes les situations se comprennent vite ; à 100, limiter les cadres et conserver un défilement commun |

Le troisième concept doit privilégier des **sections de lignes**, même s’il est présenté comme un « board ». Cinq colonnes de cartes à 120 caractères offriraient moins de 24 caractères par groupe avant bordures : insuffisant pour un chemin et `+3 ~2 -1`. Une grille plaît visuellement mais dégrade précisément la lecture de nombreuses stacks.

## Recommandation

Prototyper les trois avec les mêmes données ; choisir la table pleine largeur comme référence de comparaison globale et l’arbre annoté comme option conservant le mieux les repères actuels. Le regroupement par impact mérite son prototype pour répondre à « où dois-je regarder d’abord ? ».

Invariants des trois essais : compteurs seulement après la fin du plan/apply ; zéro explicite pour un résultat terminé sans changement ; absence de compteurs sur erreur, annulation ou exécution en cours ; état d’exécution toujours distinguable. Une stack ayant des suppressions et des ajouts apparaît une seule fois dans « suppressions », avec ses trois valeurs. Le groupe « autres changements » exclut alors les suppressions. Les chemins restent accessibles et la liste ne dépend jamais du Target sélectionné dans Output. Le contrat clavier/focus doit être décidé avant toute implémentation.
