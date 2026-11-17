# Sicherheit

## Unterstützte Versionen

Sicherheitskorrekturen gibt es nur für das jeweils neueste Release (Image-Tags `latest` / `X.Y.Z`).

## Schwachstelle melden

Bitte **kein** öffentliches Issue. Meldung vertraulich über
[GitHub Security Advisories](../../security/advisories/new). Eine erste Antwort kommt in der Regel innerhalb weniger Tage.

## Bedrohungsmodell eines Betriebs

- **Keine Anmeldung.** Wer die UI oder API erreicht, kann die Heizung steuern. Nur im Heimnetz betreiben oder hinter
  einem Reverse-Proxy mit Anmeldung.
- **Fachhandwerker-Code `17`** ist wie am Gerät eine Bediensperre, kein Zugriffsschutz.
- **Browser-Angriffe**: Schreibzugriffe mit fremdem `Origin` (CSRF) und Anfragen über unbekannte Hostnamen
  (DNS-Rebinding, siehe `ALLOWED_HOSTS`) werden abgewiesen; CSP und Sicherheits-Header sind gesetzt.
- **Schreiben** nur für Katalog-Parameter mit Wertprüfung; Werksreset, Gerätekennung und Zähler sind gesperrt.
  `READONLY=1` sperrt alle Änderungen.
- **ebusd** selbst hat keine Authentifizierung: Port `8888` nicht ins Internet freigeben.
- **Container**: statisches Binary auf `scratch`, Nutzer 6000, läuft mit `read_only`, ohne Capabilities und mit
  `no-new-privileges`.
