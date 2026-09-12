# Reconnaître les résumés OpenTofu et Terragrunt

**OpenTofu peut partager le modèle de comptes Terraform ; Terragrunt demande une couche de lecture de son enveloppe de logs.** Une unité Terragrunt par Target Runny est le cas le plus directement attribuable. Plusieurs unités dans un Target demandent un contrat de restitution par unité ou d'agrégation ; cette recherche ne décide pas de les exclure.

Recherche du 12 septembre 2026 pour [Vérifier les résumés OpenTofu et Terragrunt](https://github.com/theopoc/runny/issues/99), dans [Afficher les changements Terraform, OpenTofu et Terragrunt par Target dans Runny](https://github.com/theopoc/runny/issues/93). Complément au [rapport Terraform](2026-09-11-terraform-change-summary.md). Context7 a été interrogé pour les deux projets, puis les résultats vérifiés dans leurs documentations et sources officielles. Versions inspectées : **OpenTofu v1.12.6 et Terragrunt v1.1.4**. Aucun binaire `tofu` ou `terragrunt` disponible localement : aucune installation, exécution d'infrastructure ou fixture prétendument capturée. Les exemples ci-dessous sont issus des formats du code, pas d'une exécution locale.

## OpenTofu : comptes compatibles, enveloppe et variantes propres

Les formes principales sont conservées, après retrait des séquences ANSI :

```text
Plan: 3 to add, 2 to change, 1 to destroy.
Apply complete! Resources: 3 added, 2 changed, 1 destroyed.
No changes. Your infrastructure matches the configuration.
```

Le rendu connaît aussi les variantes sans changements de refresh-only/destroy et omet le résumé numérique pour certains changements uniquement d'outputs. Un remplacement contribue aux créations et destructions. Les trois nombres ne décrivent donc ni les attributs ni tous les changements de state. [Rendu OpenTofu](https://github.com/opentofu/opentofu/blob/v1.12.6/internal/command/jsonformat/plan.go#L124-L290), [rendu apply](https://github.com/opentofu/opentofu/blob/v1.12.6/internal/command/views/apply.go#L131-L178).

**Différence supplémentaire à couvrir :** v1.12.6 accepte un compteur d'import avant les trois comptes et un compteur d'oubli après eux : `, N to forget.` dans le plan, `, N forgotten.` dans l'apply. Le JSON contient `import` et `forget`, séparés de `remove`. Ne pas assimiler un oubli à une destruction, ni rejeter toute ligne dont le point ne suit pas immédiatement `destroy`. [Structure exacte](https://github.com/opentofu/opentofu/blob/v1.12.6/internal/command/views/json/change_summary.go#L21-L60).

Le JSON UI de `tofu plan/apply -json` reste un flux d'objets par ligne : `type=change_summary`, `changes.add/change/remove`, `changes.operation=plan/apply/destroy`. L'émetteur est **`@module=tofu.ui`** et le message de version utilise **`tofu`**, pas `terraform`. Le texte terminal n'est pas une interface stable ; le contrat JSON prévoit la compatibilité des versions mineures et le rejet d'un majeur inconnu. [Documentation UI OpenTofu](https://opentofu.org/docs/internals/machine-readable-ui/).

Attention aux exemples documentaires anciens : la page mentionne encore `ui=1.0`, tandis que le code v1.12.6 émet **`ui=1.2`**. Valider le majeur et les champs utiles, sans coder l'égalité stricte à `1.0`. [Version réellement émise](https://github.com/opentofu/opentofu/blob/v1.12.6/internal/command/views/json_view.go#L21-L74).

`tofu plan -detailed-exitcode` conserve `0=diff vide`, `1=erreur`, `2=diff non vide`. Un résumé plan ne prouve pas un apply ; le résumé apply est publié après une opération réussie. Conserver les règles Terraform : inconnu distinct de zéro, compte planifié distinct du compte appliqué, aucune déduction à partir du seul statut shell. [Options plan](https://opentofu.org/docs/cli/commands/plan/#other-options), [garde du résumé apply](https://github.com/opentofu/opentofu/blob/v1.12.6/internal/command/apply.go#L125-L149).

## Terragrunt : commandes et unités

`terragrunt plan` et `terragrunt apply` sont des raccourcis documentés. `terragrunt run -- plan` et `terragrunt run -- apply` sont leurs formes explicites ; `--` sépare les arguments Terragrunt des arguments du moteur. Terragrunt orchestre OpenTofu **ou** Terraform : son nom seul ne permet pas de choisir l'enveloppe JSON du moteur. [Raccourcis](https://docs.terragrunt.com/reference/cli/commands/opentofu-shortcuts/), [commande run](https://docs.terragrunt.com/reference/cli/commands/run/#usage).

`run --all` exécute la commande dans plusieurs unités découvertes. `stack run plan/apply` génère normalement la stack puis appelle le même mécanisme `runall`. L'ancien `run-all` a été remplacé par `run --all` lors de la refonte CLI ; cela n'établit pas une garantie de compatibilité de logs avec toutes les versions historiques. [Exécution d'une stack](https://docs.terragrunt.com/reference/cli/commands/stack/run/), [appel commun](https://github.com/gruntwork-io/terragrunt/blob/v1.1.4/internal/cli/commands/stack/stack.go#L118-L132), [migration CLI](https://docs.terragrunt.com/migrate/cli-redesign/#use-the-new-run-command).

## Trois formes de sortie à distinguer

| Forme | Contenu et traitement proposé |
| --- | --- |
| Texte moteur brut | Résumé OpenTofu/Terraform direct ; parseur de moteur habituel. |
| Texte Terragrunt enrichi | Préfixe avec heure, niveau, unité éventuelle et moteur ; décoder l'enveloppe avant le résumé. |
| JSON de logs Terragrunt | Objet de log avec `msg` contenant le texte ; décoder l'objet, puis le contenu de `msg`, en conservant l'unité. |

Le format pretty ressemble à `12:00:00.000 STDOUT [app] tofu: <message>`. L'unité peut manquer pour un appel local simple. Les sorties du moteur utilisent normalement les niveaux `STDOUT`/`STDERR` ; les opérations internes en mode headless utilisent `INFO`/`ERROR`. Auto-init, diagnostics Terragrunt et hooks peuvent ajouter leurs propres messages. Ne pas supprimer arbitrairement tout ce qui précède `Plan:`. [Enrichissement officiel](https://docs.terragrunt.com/reference/logging/#enrichment), [construction pretty](https://github.com/gruntwork-io/terragrunt/blob/v1.1.4/pkg/log/format/format.go#L41-L72).

Le preset JSON du **code v1.1.4** expose `time`, `level`, `working-dir`, `tf-path`, `tf-command-args` et `msg`, selon les métadonnées disponibles. L'exemple de la documentation du formatter présente encore `prefix` : il faut distinguer les versions/formats reconnus. Les formats personnalisés peuvent supprimer ou transformer les métadonnées ; leur lecture universelle ne peut être promise. [Preset JSON du code](https://github.com/gruntwork-io/terragrunt/blob/v1.1.4/pkg/log/format/format.go#L75-L112), [formats documentés](https://docs.terragrunt.com/reference/logging/formatting/#json).

**`--log-format json` n'active pas le JSON du moteur.** Sans `-json` moteur, Terragrunt enveloppe ses sorties dans ses logs. Avec `-json` moteur, le chemin v1.1.4 le transmet directement, même si le format des logs Terragrunt est JSON. Il faut alors reconnaître plusieurs familles d'objets dans la sortie capturée. `--tf-forward-stdout`, ou le preset `bare`, permet également le passage brut ; aucune de ces options ne doit être injectée par Runny. [Sélection de l'enveloppe et passage brut](https://github.com/gruntwork-io/terragrunt/blob/v1.1.4/internal/tf/run_cmd.go#L127-L230).

Autre détail du code : en JSON de logs, `msg` peut contenir un fragment ou plusieurs lignes d'une écriture du moteur. Décoder les nouvelles lignes échappées et conserver l'analyse incrémentale par unité ; ne pas supposer qu'un objet log correspond toujours à une ligne Terraform entière. [Écriture des messages](https://github.com/gruntwork-io/terragrunt/blob/v1.1.4/pkg/log/writer/writer.go#L43-L80).

Les anciens flags `--terragrunt-json-log` et `--terragrunt-tf-logs-to-json` sont attestés par l'annonce de 2024. Ils constituent une famille historique à couvrir par fixtures spécifiques si requise, sans les confondre avec le preset actuel. [Annonce Gruntwork](https://www.gruntwork.io/blog/new-terragrunt-features-graph-structured-logging-telemetry).

## Statut final et exécution de plusieurs unités

En v1.1.4, le code du moteur avec `-detailed-exitcode` est mémorisé par chemin d'unité. Un deux permet au déroulement Terragrunt de continuer ; le statut final peut alors rester deux si aucune autre erreur ne survient. [Capture du statut](https://github.com/gruntwork-io/terragrunt/blob/v1.1.4/internal/tf/run_cmd.go#L107-L124), [sortie CLI](https://github.com/gruntwork-io/terragrunt/blob/v1.1.4/internal/cli/exit.go#L16-L62).

Pour `run --all`, les codes conventionnels s'agrègent ainsi : **une erreur unitaire à 1 domine ; sinon un 2 domine ; sinon 0**. Cela ne donne aucun nombre de ressources. Le code actuel prévoit aussi les codes supérieurs à deux comme erreurs et en retient le maximum. [Règle documentée](https://docs.terragrunt.com/reference/cli/commands/run/#all), [algorithme v1.1.4](https://github.com/gruntwork-io/terragrunt/blob/v1.1.4/internal/tf/detailed_exitcode.go#L50-L92).

Les erreurs de before/after hooks sont jointes au résultat de l'action : un résumé moteur réussi peut donc précéder un échec Terragrunt. Un refus de la confirmation destructive `run --all` peut même sortir à zéro sans opération. Ni zéro ni deux ne suffisent à attribuer une réussite à un résumé. [Hooks](https://github.com/gruntwork-io/terragrunt/blob/v1.1.4/internal/runner/run/run.go#L433-L488), [annulation](https://github.com/gruntwork-io/terragrunt/blob/v1.1.4/internal/cli/exit.go#L42-L62).

**Déduction pour Runny :** prendre simplement le dernier résumé d'un `run --all` ferait passer la dernière unité pour le Target entier. Une somme peut avoir un sens si Runny conserve unité, phase, tentative et complétude, sans additionner plan+apply ni les répétitions d'une unité. Un flux brut concurrent peut perdre l'identité nécessaire ; le JSON UI moteur seul n'a pas de clé Terragrunt d'unité. Dans ce cas, une agrégation exhaustive n'est pas déductible du seul stdout. Choisir un indicateur partiel, un total contrôlé ou une autre représentation reste une décision de produit ouverte.

## Matrice de cadrage

« Supportable » signifie étayé par ces sources, pas implémenté ni validé dans Runny.

| Entrée | Cadrage proposé | Limite |
| --- | --- | --- |
| `tofu plan/apply`, v1.12.6 | Supportable, texte et JSON UI | ANSI, `import`, `forget`, versions JSON ; pas de conversion plan vers appliqué. |
| `terragrunt plan/apply` ou `run -- plan/apply`, v1.1.4, une unité | Supportable après normalisation | Le statut Terragrunt inclut aussi ses hooks et erreurs propres. |
| Pretty ou JSON log actuel | Supportable avec enveloppe reconnue | Unité absente possible ; `msg` fragmenté ou multiligne. |
| `-json` moteur ou stdout brut | Supportable pour une unité identifiable | Les logs Terragrunt peuvent coexister dans la capture. |
| `run --all`, `stack run`, plusieurs unités | Contrat de restitution par unité ou d'agrégation nécessaire | Dernier résumé insuffisant ; identité, reprises et résultats partiels indispensables. |
| Ancien `run-all` et anciens flags JSON | Famille documentée à fixture séparée | Aucune couverture de toutes les versions 0.x démontrée. |
| Format personnalisé, lignes préfixées inconnues | Résumé indéterminé | Ne pas deviner l'unité ou supprimer librement les préfixes. |
| Erreur, annulation, doublon, redémarrage | Conserver provenance et état du run | Ne pas présenter un compte incomplet comme résultat global réussi. |

Les tests futurs devront vérifier ces familles avec des captures des versions effectivement visées. Aucun changement de commande, flag, backend ou configuration de logs ne découle de cette recherche.
