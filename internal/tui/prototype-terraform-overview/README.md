# PROTOTYPE — Terraform stack overview

Question: how can Runny show the Terraform result of every Target in a shared overview, without opening each Target's Output?

This is a disposable, browser-rendered terminal sketch with synthetic data. It does not execute Terraform, modify Runny, or persist UI state. It is not a Bubble Tea implementation or a Ghostty capture.

## Run

From the repository root:

```sh
python3 -m http.server 8000 --bind 127.0.0.1 --directory internal/tui/prototype-terraform-overview
```

Open `http://127.0.0.1:8000/?variant=B`. The document can also be opened directly in a browser.

## Alternatives

- **A — Annotated tree:** preserve Tasks and the existing split with Output at 100 columns or more. Add compact counts to each Target. In a small Tasks panel, prioritize counts and shorten the status while hiding duration. Parent directories do not sum resource counts.
- **B — Full-width table:** one row per Target, explicit numeric columns for additions, changes and removals. Use the available width for comparison, with Output left to the existing detail flow. This is the recommended candidate for the user's global comparison goal.
- **C — Grouped impact list:** organize the same Targets into mutually exclusive sections, with errors/cancellations and deletions visible first. Every Target appears once. Section headings count Targets, not resource totals. This grouping is a design proposal, not an existing Runny feature.

Switch variants with the bottom bar or `?variant=A`, `B`, or `C`. Prototype-only controls offer 120×30, 100×24, 80×24 and 60×24 terminal sizes; 12 or 48 synthetic stacks; plan/apply; monochrome; and a 36-column Tasks panel. The up/down controls page through the synthetic rows. These controls do not propose new Runny keybindings.

Counters are shown only on completed successful examples. Running, queued, failed and cancelled examples have no counters. Completed examples with unavailable summaries use a dash, distinct from confirmed zero resource changes. The global phase label identifies planned or applied resource changes.

## Evidence and limits

- Primary-source comparison: [TUI patterns](../../../docs/research/2026-09-12-terraform-overview-tui-patterns.md).
- Browser inspection covered all three variants at four terminal sizes with 48 stacks: 12 combinations, correct row counts, and no horizontal overflow.
- Additional inspection covered the minimum 36-column Tasks panel, four-digit counters at 60 columns, monochrome, apply labels, paging, and a 320-pixel browser window showing the minimum-size state without page overflow.
- JavaScript syntax checked with `node --check`; HTML fragment read back; no network dependencies.
- These checks validate the sketch only. Production integration will still require the repository's Go tests and real-terminal inspection.
- Sorting, filters, persistence in History, multiple Terraform subcommands and process exit-code policy remain outside this visual sketch.
- No variant has been accepted yet. Keep the design ticket open until the user chooses or requests changes.

Context: [Choisir l'affichage compact des changements Terraform](https://github.com/theopoc/runny/issues/96), within [Afficher les changements Terraform par Target dans Runny](https://github.com/theopoc/runny/issues/93).
