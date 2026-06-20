package streambot

import (
	"os"
	"path/filepath"
	"strings"
)

const DefaultResponsesDir = "data/streambot/responses"

var fallbackResponses = map[string]string{
	"socials":          "Social links are not configured yet.",
	"valorant":         "Valorant queue is live. Drop !crosshair for my crosshair/sens.",
	"sky":              "Sky: Children of the Light is live. Come fly with us.",
	"generic":          "Chill stream is live. Drop !specs for setup and hang out.",
	"specs":            "Specs: R9 7950X | RTX 4070 | 32GB DDR5 6000 | 990 Pro 1TB | ROG B650E-F | RM850e | 360mm AIO | MSI 24 300Hz + LG 27 180Hz | Viper Mini | BlackShark V2",
	"crosshair":        "Valorant: Crosshair 0;s;1;P;h;0;0t;1;0l;7;0v;0;0g;1;0o;1;0a;1;0f;0;1t;3;1l;0;1v;4;1g;1;1o;1;1a;1;1m;0;1f;0;S;c;1;s;0.543 | Sens: 0.2 @ 1200dpi (240 eDPI)",
	"isekai":           "Sky Companion Overlay link is in the stream description. It has timers, events, realms, and cozy Sky stream helpers. ✨",
	"welcome_valorant": "Hey there, welcome to Valorant stream! Chill aim, clean comms, clutch vibes. Drop !crosshair or !specs for setup. 🎯",
	"welcome_sky":      "Hey there, welcome to Sky stream! Come fly, chill, and vibe with us. ✨ Sky Companion Overlay link is in the description.",
	"welcome_generic":  "Hey there, welcome to stream! Grab a seat, say hi, and vibe with us. Drop !specs for setup. ✨",
}

type Responses struct {
	dir string
}

func NewResponses(dir string) Responses {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		dir = DefaultResponsesDir
	}
	return Responses{dir: dir}
}

func (r Responses) Text(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return ""
	}

	body, err := os.ReadFile(filepath.Join(r.dir, name+".txt"))
	if err == nil {
		if text := strings.TrimSpace(string(body)); text != "" {
			return text
		}
	}

	return fallbackResponses[name]
}
