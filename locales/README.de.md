# ADB GUI 🚀

Ein leistungsstarkes, modernes und eigenständiges Android-Verwaltungstool, entwickelt mit **Wails**, **React** und **Ant Design**.

> ✨ **Hinweis**: Diese Anwendung ist das Ergebnis von reinem **vibecoding**.

![License](https://img.shields.io/badge/license-MIT-blue.svg)
![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Windows%20%7C%20Linux-lightgrey.svg)

## ✨ Funktionen

### 📱 Geräteverwaltung
- **Einheitliche Geräteliste**: Verwalten Sie physische und drahtlose Geräte (USB/Wi-Fi-Merging) nahtlos in einer einzigen Ansicht.
- **Drahtlose Verbindung**: Verbinden Sie sich mühelos per IP/Port-Pairing mit mDNS-Unterstützung.
- **Gerätehistorie**: Schneller Zugriff auf zuvor verbundene Offline-Geräte.
- **Geräte-Pinning**: Heften Sie Ihr meistgenutztes Gerät an, damit es immer ganz oben in der Liste steht.
- **Sequentieller Refresh**: Intelligenterer sequentieller Polling-Mechanismus für eine stabile und flimmerfreie UI.

### 📦 App-Verwaltung
- **Volle Paketkontrolle**: Installieren (Drag & Drop), Deinstallieren, Aktivieren, Deaktivieren, Stoppen erzwingen, Daten löschen.
- **APK-Verwaltung**: Exportieren installierter APKs, Batch-Installation.
- **Intelligente Filterung**: Suchen und Filtern nach System-/Benutzer-Apps.
- **Schnellaktionen**: Starten Sie Apps oder springen Sie direkt zu deren Protokollen.

### 🖥️ Bildschirmspiegelung (Scrcpy)
- **Hohe Leistung**: Spiegelung mit geringer Latenz powered by Scrcpy.
- **Aufnahme**: Unabhängige Hintergrundaufnahme mit Unterstützung für mehrere Geräte gleichzeitig und Ein-Klick-Ordnerzugriff.
- **Audio-Weiterleitung**: Streamen Sie Geräteaudio auf Ihren Computer (Android 11+).
- **Anpassung**: Passen Sie Auflösung, Bitrate, FPS und Codec (H.264/H.265) an.
- **Steuerung**: Multi-Touch-Unterstützung, Wach bleiben, Bildschirm-Aus-Modus.

### 📂 Dateiverwaltung
- **Voll ausgestatteter Explorer**: Durchsuchen, Kopieren, Ausschneiden, Einfügen, Umbenennen, Löschen und Erstellen von Ordnern.
- **Drag & Drop**: Laden Sie Dateien hoch, indem Sie sie einfach in das Fenster ziehen.
- **Downloads**: Einfache Dateiübertragung vom Gerät auf den Computer.
- **Vorschau**: Öffnen Sie Dateien direkt auf dem Host-Computer mit Standardanwendungen.

### 📜 Erweitertes Logcat
- **Echtzeit-Streaming**: Live-Protokollansicht mit automatischer Scroll-Steuerung.
- **Leistungsstarke Filterung**: Filtern nach Protokollebene, Tag, PID oder benutzerdefiniertem Regex.
- **Vorfilterung (Pre-Filter)**: Hochleistungs-Buffering, das nur Protokolle speichert, die bestimmten Regeln entsprechen.
- **App-Zentriert**: Automatisches Filtern von Protokollen für eine bestimmte Anwendung.

### 💻 ADB Shell
- **Integrierte Konsole**: Führen Sie rohe ADB-Befehle direkt in der App aus.
- **Befehlsverlauf**: Schneller Zugriff auf zuvor ausgeführte Befehle.

### 🔌 Systemablage
- **Schnellzugriff**: Steuern Sie die Spiegelung und sehen Sie den Gerätestatus über die Menüleiste / Systemablage.
- **Geräte-Pinning**: Heften Sie Ihr Hauptgerät an, damit es oben in der Liste und im Menü der Systemablage erscheint.
- **Tray-Funktionen**: Direkter Zugriff auf Logcat, Shell und Dateimanager für angeheftete Geräte über die Systemablage.
- **Aufnahme-Indikatoren**: Visueller roter Punkt in der Systemablage, wenn eine Aufnahme aktiv ist.
- **Hintergrundbetrieb**: Lassen Sie die App im Hintergrund laufen, um sofortigen Zugriff zu erhalten.

---

## 🛠️ Integrierte Binärdateien

Diese Anwendung ist vollständig eigenständig. Sie enthält:
- **ADB** (Android Debug Bridge)
- **Scrcpy** ausführbare Datei
- **Scrcpy-server**

Beim Start werden diese in ein temporäres Verzeichnis extrahiert und automatisch verwendet. Sie müssen Ihren System-PATH nicht konfigurieren.

---

## ⚠️ Wichtige Hinweise für Xiaomi/Poco/Redmi-Benutzer

Um die **Touch-Steuerung** in Scrcpy zu aktivieren, müssen Sie:
1. Zu den **Entwickleroptionen** gehen.
2. **USB-Debugging** aktivieren.
3. **USB-Debugging (Sicherheitseinstellungen)** aktivieren.
   *(Hinweis: Dies erfordert bei den meisten Xiaomi-Geräten eine SIM-Karte und eine Anmeldung im Mi-Konto).*

---

## 🚀 Erste Schritte

### Voraussetzungen
- **Go** (v1.21)
- **Node.js** (v18 LTS)
- **Wails CLI** (v2.9.2)
  ```bash
  go install github.com/wailsapp/wails/v2/cmd/wails@v2.9.2
  ```

### Entwicklung
```bash
wails dev
```

### Build
```bash
wails build
```
Die kompilierte Anwendung wird in `build/bin` verfügbar sein.

### Release
Dieses Projekt verwendet GitHub Actions, um Multi-Plattform-Builds zu automatisieren. So erstellen Sie ein neues Release:
1. Taggen Sie Ihren Commit: `git tag v1.0.0`
2. Pushen Sie den Tag: `git push origin v1.0.0`
Die GitHub Action wird automatisch für macOS, Windows und Linux bauen und die Artefakte auf die Release-Seite hochladen.

---

## 🔧 Fehlerbehebung

### macOS: "App ist beschädigt und kann nicht geöffnet werden"
Wenn Sie die App von GitHub herunterladen und den Fehler *"adbGUI.app ist beschädigt und kann nicht geöffnet werden"* sehen, liegt dies an der macOS Gatekeeper Quarantäne.

Um dies zu beheben, führen Sie folgenden Befehl im Terminal aus:
```bash
sudo xattr -cr /path/to/adbGUI.app
```
*(Ersetzen Sie `/path/to/adbGUI.app` durch den tatsächlichen Pfad zu Ihrer heruntergeladenen Anwendung)*

> **Oder selbst kompilieren:** Wenn Sie Gatekeeper nicht umgehen möchten, können Sie die [App ganz einfach lokal aus dem Quellcode kompilieren](#-erste-schritte). Das dauert nur wenige Minuten!

### Windows: "Der Computer wurde durch Windows geschützt"
Wenn ein blaues SmartScreen-Fenster den Start verhindert:
1. Klicken Sie auf **Weitere Informationen (More info)**.
2. Klicken Sie auf **Trotzdem ausführen (Run anyway)**.

---

## 📄 Lizenz
Dieses Projekt ist unter der MIT-Lizenz lizenziert.
