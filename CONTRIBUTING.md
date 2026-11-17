# Mitmachen

Vor einem PR:

```bash
gofmt -l .        # muss leer sein
go vet ./...
go test -race ./...
```

Die CI führt zusätzlich die Playwright-Browsertests gegen das simulierte ebusd (`cmd/fakeebusd`), Hadolint,
Trivy und einen Docker-Smoke-Test aus.

- **Neue Parameter** kommen in `internal/catalog/catalog.go` (Name aus
  [`ebusd-configuration`](https://github.com/john30/ebusd-configuration), deutsche Bezeichnung, Grenzen, `.W()` nur
  wenn das Schreiben ungefährlich ist). `TestCatalogConsistency` prüft die Einträge.
- **Testwerte** stehen in `e2e/fixtures/values.tsv` (Rohwerte wie von `ebusctl read`); Seriennummern vorher entfernen.
- Schreibende Tests **nie gegen eine echte Heizung** laufen lassen.
- UI-Änderungen: Screenshots mit `e2e/shots.py` neu erzeugen, wenn sich das Aussehen ändert.
