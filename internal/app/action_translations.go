package app

import "android-toolbox/internal/actions"

// builtinCategoryTranslationsDE maps a built-in action's English Category
// value to its German translation. Only categories that actually read
// differently are listed - Logs/Shell/Apps are the same word in both.
var builtinCategoryTranslationsDE = map[string]string{
	"Files":   "Dateien",
	"Device":  "Gerät",
	"Network": "Netzwerk",
	"Display": "Anzeige",
}

// actionTranslation holds one built-in action's German Name/Description,
// plus its Params' translated labels (keyed by Param.Name).
type actionTranslation struct {
	Name        string
	Description string
	Params      map[string]string
}

// builtinActionTranslationsDE maps a built-in action's ID (see
// internal/actions/actions.default.yaml, the authoritative English source)
// to its German translation. Only the actions shipped by default are
// covered here - user-created actions (via the AI feature, or hand-edited
// into actions.yaml) have no translation and always display exactly as
// written, in whatever language they were authored in.
var builtinActionTranslationsDE = map[string]actionTranslation{
	"logcat-snapshot": {
		Name:        "Logs anzeigen (Snapshot)",
		Description: "Zeigt die letzten 500 Logcat-Zeilen des Geräts an",
	},
	"logcat-live": {
		Name:        "Logs live verfolgen",
		Description: "Streamt Logcat live, bis mit Strg+C abgebrochen wird",
	},
	"logcat-clear": {
		Name:        "Log-Puffer leeren",
		Description: "Löscht den Logcat-Puffer auf dem Gerät",
	},
	"shell-open": {
		Name:        "Interaktive Shell",
		Description: "Öffnet eine interaktive adb-Shell auf dem Gerät",
	},
	"shell-dumpsys-battery": {
		Name:        "Akku-Status (dumpsys)",
		Description: "Zeigt den ausführlichen Akku-Status",
	},
	"shell-dumpsys-notifications": {
		Name:        "Aktive Benachrichtigungen",
		Description: "Zeigt aktive Benachrichtigungen über dumpsys an",
	},
	"file-pull": {
		Name:        "Datei vom Gerät holen",
		Description: "Kopiert eine Datei vom Gerät auf den PC",
		Params: map[string]string{
			"remote_path": "Pfad auf dem Gerät",
			"local_path":  "Zielordner auf dem PC",
		},
	},
	"file-push": {
		Name:        "Datei auf Gerät übertragen",
		Description: "Kopiert eine Datei vom PC auf das Gerät",
		Params: map[string]string{
			"local_path":  "Datei auf dem PC",
			"remote_path": "Zielpfad auf dem Gerät",
		},
	},
	"screenshot": {
		Name:        "Screenshot ziehen",
		Description: "Erstellt einen Screenshot und speichert ihn lokal",
	},
	"screenrecord-start": {
		Name:        "Bildschirmaufnahme starten (30s)",
		Description: "Nimmt 30 Sekunden den Bildschirm auf und holt die Datei danach ab",
	},
	"apps-list": {
		Name:        "Installierte Pakete auflisten",
		Description: "Listet alle installierten App-Pakete auf",
	},
	"apps-install": {
		Name:        "APK installieren",
		Description: "Installiert eine lokale APK-Datei auf dem Gerät",
		Params: map[string]string{
			"apk_path": "Pfad zur APK-Datei",
		},
	},
	"apps-uninstall": {
		Name:        "App deinstallieren",
		Description: "Deinstalliert ein Paket vom Gerät",
		Params: map[string]string{
			"package": "Package-Name",
		},
	},
	"apps-clear-data": {
		Name:        "App-Daten leeren",
		Description: "Setzt eine App auf den Auslieferungszustand zurück",
		Params: map[string]string{
			"package": "Package-Name",
		},
	},
	"device-info": {
		Name:        "Geräteinformationen",
		Description: "Modell, Android-Version, SDK und Build-Infos",
	},
	"device-reboot": {
		Name:        "Neustart",
		Description: "Startet das Gerät normal neu",
	},
	"device-reboot-recovery": {
		Name:        "Neustart in Recovery",
		Description: "Startet das Gerät im Recovery-Modus neu",
	},
	"device-reboot-bootloader": {
		Name:        "Neustart in Bootloader",
		Description: "Startet das Gerät im Bootloader/Fastboot-Modus neu",
	},
	"wifi-adb-enable": {
		Name:        "WLAN-ADB aktivieren",
		Description: "Schaltet adb auf TCP/IP um (Port 5555) für die kabellose Verbindung",
	},
	"wifi-adb-connect": {
		Name:        "Per WLAN verbinden",
		Description: "Verbindet sich mit einem Gerät über dessen IP-Adresse",
		Params: map[string]string{
			"ip": "IP-Adresse des Geräts",
		},
	},
	"port-forward": {
		Name:        "Port-Forwarding einrichten",
		Description: "Leitet einen lokalen Port auf einen Geräte-Port um",
		Params: map[string]string{
			"local_port":  "Lokaler Port",
			"remote_port": "Port auf dem Gerät",
		},
	},
	"scrcpy-display": {
		Name:        "Display spiegeln (Standard)",
		Description: "Öffnet scrcpy mit den Standardeinstellungen",
	},
	"scrcpy-no-audio": {
		Name:        "Display spiegeln (ohne Audio)",
		Description: "Spiegelt das Display ohne Audio-Weiterleitung",
	},
	"scrcpy-audio-only": {
		Name:        "Nur Audio weiterleiten",
		Description: "Leitet nur den Geräte-Ton weiter, ohne Bild",
	},
	"scrcpy-record": {
		Name:        "Display spiegeln und aufnehmen",
		Description: "Spiegelt das Display und zeichnet die Sitzung als mp4 auf",
	},
	"scrcpy-otg": {
		Name:        "OTG-Modus (nur Maus/Tastatur)",
		Description: "Nutzt das Gerät als reines USB-HID-Ziel, ohne Bildspiegelung",
	},
}

// localizeAction returns a. Category is left untouched here - that's
// resolved separately at the category-group level (see categoryDisplayLabel
// in screen_dashboard.go), since one category groups several actions and
// only needs translating once, not per action.
//
// Only applies when the UI language is German and a is a recognized
// built-in action; anything else - including every user-created action,
// regardless of language - passes through completely unchanged.
func localizeAction(a actions.Action, t uiText) actions.Action {
	if t.LanguageCode != "de" {
		return a
	}
	tr, ok := builtinActionTranslationsDE[a.ID]
	if !ok {
		return a
	}
	a.Name = tr.Name
	a.Description = tr.Description
	if len(tr.Params) > 0 && len(a.Params) > 0 {
		params := make([]actions.Param, len(a.Params))
		copy(params, a.Params)
		for i, p := range params {
			if label, ok := tr.Params[p.Name]; ok {
				p.Label = label
				params[i] = p
			}
		}
		a.Params = params
	}
	return a
}
