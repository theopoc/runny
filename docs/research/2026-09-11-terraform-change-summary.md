# Extraire les comptes Terraform par Target

**Conclusion : la reconnaissance passive des résumés Terraform permet d'afficher `+3 ~2 -1`, à condition de conserver leur provenance plan/apply et une valeur « inconnue » distincte de zéro.** Le texte humain constitue une heuristique de compatibilité ; le JSON UI, lorsqu'il est déjà demandé par l'utilisateur, fournit un contrat versionné. Aucune commande supplémentaire ni aucun changement d'arguments n'est nécessaire à cette approche.

Recherche du 11 septembre 2026 pour [Identifier les signaux fiables des résumés Terraform plan et apply](https://github.com/theopoc/runny/issues/94), rattaché à [Afficher les changements Terraform par Target dans Runny](https://github.com/theopoc/runny/issues/93). Context7 a fourni les extraits, ensuite vérifiés chez HashiCorp. Le code source et les essais locaux concernent **Terraform 1.14.3**, `darwin_arm64` ; aucune compatibilité universelle n'en découle.

## Signaux vérifiés

### Texte humain

Après suppression des codes de présentation, les formes principales sont :

```text
Plan: 3 to add, 2 to change, 1 to destroy.
Apply complete! Resources: 3 added, 2 changed, 1 destroyed.
No changes. Your infrastructure matches the configuration.
```

`Plan:` décrit les opérations prévues. `Apply complete!` décrit le compte final publié après un apply réussi. La troisième forme établit zéro changement de ressources pour cette phase de plan ; elle peut aussi apparaître pendant un apply. Les nombres comptent des **opérations sur les ressources**, pas les attributs modifiés. Les rendus Terraform insèrent des séquences ANSI au milieu des phrases et autour des compteurs. [Rendu du plan](https://github.com/hashicorp/terraform/blob/v1.14.3/internal/command/jsonformat/plan.go#L124-L152), [résumé apply](https://github.com/hashicorp/terraform/blob/v1.14.3/internal/command/views/apply.go#L63-L84).

Variantes à reconnaître explicitement :

- Un import ajoute `N to import,` immédiatement après `Plan:` et `N imported,` après `Resources:`. L'import reste distinct des créations ; un import suivi d'un remplacement peut compter simultanément dans import, add et destroy. Le workflow d'import par configuration existe depuis Terraform 1.5. [Tutoriel officiel d'import](https://developer.hashicorp.com/terraform/tutorials/cli/state-import).
- En 1.14.3, un suffixe `Actions: N to invoke.` ou `Actions: N invoked.` peut suivre le point final. Il ne contribue pas aux trois comptes de ressources. Les champs JSON correspondants existent aussi. [Structure et formats des résumés](https://github.com/hashicorp/terraform/blob/v1.14.3/internal/command/views/json/change_summary.go#L19-L55).
- Les messages sans changements diffèrent en mode refresh-only et destroy. Leur reconnaissance doit suivre une liste de formulations vérifiées, sans traiter toute phrase commençant par « No » comme zéro. [Branches du rendu humain](https://github.com/hashicorp/terraform/blob/v1.14.3/internal/command/jsonformat/plan.go#L100-L152).

Un remplacement compte **une création et une destruction**, avec zéro modification sur place pour cette opération. Les hooks apply distinguent création, modification, suppression et import ; le remplacement incrémente les deux côtés. Il ne faut donc pas convertir un remplacement en `~1`. [Comptage effectif](https://github.com/hashicorp/terraform/blob/v1.14.3/internal/command/views/hook_count.go#L63-L119).

HashiCorp indique explicitement que le texte terminal n'est **pas une interface stable** d'intégration. Une reconnaissance exacte et couverte par des fixtures reste utile, mais une sortie absente ou différente doit donner « inconnu ». [Contrat des sorties UI](https://developer.hashicorp.com/terraform/internals/machine-readable-ui#introduction).

### JSON déjà présent dans la sortie

`terraform plan -json` et `terraform apply -json` produisent un objet JSON par ligne. Le premier message `type=version` expose la version Terraform et celle du schéma `ui`. Accepter les versions mineures compatibles, ignorer les propriétés inconnues et refuser un majeur non pris en charge. Pour `type=change_summary`, lire les entiers `changes.add`, `changes.change`, `changes.remove` et `changes.operation` (`plan`, `apply` ou `destroy`), plutôt que le texte `@message`. `@module=terraform.ui` renforce la reconnaissance. [Messages et versionnement JSON UI](https://developer.hashicorp.com/terraform/internals/machine-readable-ui#change-summary).

En 1.14.3, `import` et `action_invocation` sont des dimensions supplémentaires. La synthèse JSON du plan compte séparément les imports et les remplacements ; les messages de dérive et les lectures de data sources ne doivent pas devenir des créations/modifications supplémentaires. [Construction du résumé JSON](https://github.com/hashicorp/terraform/blob/v1.14.3/internal/command/views/operation.go#L215-L273).

**`terraform show -json` est un autre format** : un document complet représentant un plan sauvegardé ou un état, avec `format_version`. Un état ne décrit pas les changements d'un apply. Dans un plan, `resource_changes[].change.actions` peut contenir `create`, `update`, `delete`, ou les deux actions d'un remplacement ; `output_changes`, `resource_drift` et `importing` sont distincts. Une éventuelle prise en charge exige un parseur séparé et ne doit jamais étiqueter ces comptes comme appliqués. Elle n'est pas nécessaire à la reconnaissance du flux plan/apply. [Commande show](https://developer.hashicorp.com/terraform/cli/commands/show#json-output), [format JSON des plans](https://developer.hashicorp.com/terraform/internals/json-format#plan-representation).

Il faut conserver les commandes de l'utilisateur telles quelles. Forcer `-json` changerait aussi l'interactivité : ce flag implique `-input=false`, et apply exige alors un plan sauvegardé ou `-auto-approve`. [Options plan](https://developer.hashicorp.com/terraform/cli/commands/plan#other-options), [options apply](https://developer.hashicorp.com/terraform/cli/commands/apply#apply-options).

## Ce que les compteurs ne prouvent pas

Un apply sans fichier commence par un plan ; un apply de plan sauvegardé exécute le plan fourni. Sur le chemin local 1.14.3, ce dernier n'imprime pas de résumé global de plan préalable : le résumé apply final suffit, sans dépendre d'un ancien run. [Modes d'apply](https://developer.hashicorp.com/terraform/cli/commands/apply#saved-plan-mode), [chemin du plan sauvegardé](https://github.com/hashicorp/terraform/blob/v1.14.3/internal/backend/local/backend_apply.go#L92-L238).

Un **plan partiel en erreur peut néanmoins être affiché**, avant les diagnostics. Un `Plan:` ou `change_summary(operation=plan)` n'est donc pas à lui seul une preuve de succès. [Publication d'un plan partiel](https://github.com/hashicorp/terraform/blob/v1.14.3/internal/backend/local/backend_plan.go#L216-L223).

Un apply qui échoue peut déjà avoir modifié les ressources et le state ; Terraform ne revient pas automatiquement en arrière. Sur le chemin local, le résumé final n'est rendu qu'après un résultat d'opération réussi. Un apply interrompu, refusé ou partiel peut donc laisser seulement le compte planifié. **Ne jamais convertir ce compte en compte appliqué**, ni déduire « zéro appliqué » d'une absence de résumé. [Gestion des erreurs](https://developer.hashicorp.com/terraform/cli/commands/apply#errors-during-an-apply-operation), [garde du résumé final](https://github.com/hashicorp/terraform/blob/v1.14.3/internal/command/apply.go#L117-L140).

Les comptes `0/0/0` n'impliquent pas une absence de toute modification : outputs, imports, déplacements d'adresses, modifications de state et actions peuvent exister séparément. Le rendu humain peut omettre complètement `Plan:` pour un changement d'outputs uniquement. [Séparation du rendu des outputs](https://github.com/hashicorp/terraform/blob/v1.14.3/internal/command/jsonformat/plan.go#L267-L279), [séparation des actions JSON](https://developer.hashicorp.com/terraform/internals/json-format#change-representation).

### Code de sortie

Avec **`terraform plan -detailed-exitcode`**, `0` signifie plan réussi avec diff vide, `1` une erreur et `2` un plan réussi avec diff non vide. Sans ce flag, zéro ne distingue pas ces deux réussites. Les seuls trois compteurs ne permettent pas de retrouver le diff global. [Référence officielle](https://developer.hashicorp.com/terraform/cli/commands/plan#other-options).

Le statut reçu par Runny appartient à la commande shell complète. Par exemple, `terraform plan -detailed-exitcode; true` peut terminer à zéro, tandis que `terraform plan; sh -c 'exit 2'` termine à deux pour une autre raison. Avec `&&`, un plan retournant deux empêche la commande suivante de démarrer. Les pipelines et leurs options ajoutent d'autres règles. Ces exemples découlent de la [grammaire officielle zsh](https://zsh.sourceforge.io/Doc/Release/Shell-Grammar.html#Simple-Commands-_0026-Pipelines).

**Recommandation : séparer résumé observé et statut du processus, sans réinterprétation générique de l'exit code 2.** Toute évolution du statut pour un lancement Terraform identifié nécessiterait son propre contrat ; la présence du mot `terraform` ou d'un résumé dans stdout n'identifie pas le processus qui a produit le statut final.

## Vérification locale ciblée

Essais en répertoires temporaires, ressource `terraform_data` du provider intégré Terraform et backend local, sans provider externe, provisioner ou cloud. Captures non commitées : `/tmp/terraform-summary-repro-20260911/results.json`.

| Cas observé | Commande essentielle | Résultat observé |
| --- | --- | --- |
| Output seul, `output "example" { value = "hello" }` | `plan -no-color -detailed-exitcode` | Code 2 ; section outputs ; aucun `Plan:` ni message sans changements. |
| Même configuration en JSON | `plan -json -detailed-exitcode` | Code 2 ; résumé plan `0/0/0`. |
| Application de l'output seul | `apply -no-color -auto-approve` | Code 0 ; résumé apply `0/0/0`. |
| Second plan sans changement | `plan -no-color -detailed-exitcode` | Code 0 ; message normal sans changements. |
| Une ressource `terraform_data` sauvegardée dans `tfplan` | `plan -out=tfplan`, puis `apply tfplan` | Plan `1/0/0` ; apply `1/0/0`, sans résumé plan préalable dans la sortie de l'apply. |
| Remplacement de cette ressource | `plan -replace=terraform_data.example -detailed-exitcode` | Code 2 ; plan `1/0/1`. |
| Deux ressources, seconde bloquée par une précondition évaluée après création de la première | `apply -json -auto-approve` | Code 1 ; plan `2/0/0`, un événement de création réussie, diagnostic d'erreur, **aucun résumé apply**. |
| Refus interactif avec réponse `no` | `apply -no-color` | Code 1 ; plan `1/0/0`, annulation, aucun résumé apply. |

Repro partiel : première ressource avec `input = "good"`, précondition de la seconde `terraform_data.good.output == "never"`, évaluée après création. Imports/actions : preuve par source uniquement. PTY et interruption par signal : non exécutés.

## Recommandations pour le parseur et matrice d'acceptation

Ces **propositions Runny** ne constituent pas des garanties Terraform ni des décisions de présentation.

| Entrée ou situation | Comportement recommandé |
| --- | --- |
| Résumé complet reconnu | Extraire trois entiers non négatifs et conserver phase, format et ordre d'observation. |
| Import ou suffixe d'action connu | Extraire les trois comptes ; distinguer les dimensions supplémentaires pour expliquer un zéro. |
| Message sans changements vérifié | Enregistrer zéro **ressource** pour la phase plan, sans annoncer que tout le run est sans effet. |
| Outputs seuls, sans résumé numérique | Compte inconnu ; la section outputs seule ne prouve pas zéro ressource. |
| Erreur, refus ou interruption après un plan | Garder la provenance plan et son caractère potentiellement incomplet ; aucun compte appliqué inventé. |
| JSON UI supporté | Valider enveloppe/types ; refuser comptes invalides ou débordants ; ne pas réanalyser `@message`. |
| Schéma JSON majeur inconnu, JSON malformé ou format humain inconnu | Aucun nouveau compte ; absence de crash ; sortie originale conservée. |
| ANSI et caractères UTF-8 fragmentés entre lectures | Parseur incrémental par Target/run, avec état de décodage ; une regex indépendante sur chaque chunk est insuffisante. |
| LF, CRLF, fin sans newline | Lignes logiques, fragment final à la clôture ; jamais les lignes visuelles de la TUI. |
| CR isolé, effacement terminal ou ligne anormalement longue | Politique explicite et mémoire bornée ; ignorer une ligne ambiguë ou tronquée plutôt que fabriquer une correspondance. |
| Résumés successifs | Ni addition ni déduplication par valeurs ; conserver l'ordre et définir la sélection affichée. |
| Plusieurs processus écrivant dans un même Target | Le flux seul ne garantit ni leur identité ni leurs limites ; aucune attribution fiable à une sous-commande si les sorties se mélangent. |
| Relance d'un Target | Réinitialiser l'analyse pour le nouveau run ; ne pas réutiliser un résumé ancien comme nouveau résultat. |
| Sortie très longue ou journalisation désactivée | Analyser les octets avant éviction du tampon ; définir explicitement si le résumé reste disponible sans journalisation. |
| Citations, préfixes, données d'output | Correspondances sur lignes complètes ; rejeter les préfixes inconnus et les simples sous-chaînes. |

**Limite de provenance :** tout programme peut imprimer une ligne Terraform, notamment `cat` rejouant un document. Ancres, enveloppe JSON et contexte réduisent les faux positifs sans authentifier l'émetteur. La détection reste une annotation, sans modifier les commandes/statuts ni déclencher d'actions.

Périmètre vérifié : Terraform natif plan/apply local. Destroy aide à classifier, sans élargissement décidé. OpenTofu, Terragrunt, wrappers, exécution distante HCP/TFE et versions non testées nécessitent leurs propres fixtures.

## Décisions encore ouvertes

Restent ouverts : emplacement, moment de publication, représentation plan/appliqué/inconnu, sélection parmi plusieurs résumés et History. `+3 ~2 -1` reste par Target, sans total global ni liste de ressources.
