package streambot

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

const CommandPrefix = "!"

type Message struct {
	Platform   string
	AuthorID   string
	AuthorName string
	Text       string
	IsOwner    bool
	At         time.Time
}

type Response struct {
	Text string
}

type Options struct {
	StateStore      *Store
	Responses       Responses
	StartedAt       time.Time
	CommandCooldown time.Duration
	Now             func() time.Time
}

type Bot struct {
	store     *Store
	responses Responses
	startedAt time.Time
	cooldown  time.Duration
	now       func() time.Time

	mu       sync.Mutex
	lastUsed map[string]time.Time
}

func New(opts Options) *Bot {
	now := opts.Now
	if now == nil {
		now = time.Now
	}

	startedAt := opts.StartedAt
	if startedAt.IsZero() {
		startedAt = now()
	}

	cooldown := opts.CommandCooldown
	if cooldown <= 0 {
		cooldown = 5 * time.Second
	}

	store := opts.StateStore
	if store == nil {
		store = NewStore("")
	}

	responses := opts.Responses
	if responses.dir == "" {
		responses = NewResponses("")
	}

	return &Bot{
		store:     store,
		responses: responses,
		startedAt: startedAt,
		cooldown:  cooldown,
		now:       now,
		lastUsed:  make(map[string]time.Time),
	}
}

func (b *Bot) Handle(ctx context.Context, msg Message) (Response, bool, error) {
	if err := ctx.Err(); err != nil {
		return Response{}, false, err
	}

	name, args, ok := parseCommand(msg.Text)
	if !ok {
		return Response{}, false, nil
	}

	if isOwnerCommand(name) && !msg.IsOwner {
		return Response{}, false, nil
	}
	if !msg.IsOwner && !b.allow(name) {
		return Response{}, false, nil
	}

	switch name {
	case "ping":
		return Response{Text: "pong"}, true, nil
	case "uptime":
		return Response{Text: "Stream bot uptime: " + formatDuration(b.now().Sub(b.startedAt))}, true, nil
	case "commands":
		return Response{Text: "Viewer: !game !specs !crosshair !isekai !valorant !sky !generic !socials | Owner: !ping !status !uptime !setgame"}, true, nil
	case "status":
		state, err := b.store.Load()
		if err != nil {
			return Response{}, false, err
		}
		game := "not set"
		if state.CurrentGame != "" {
			game = DisplayGame(state.CurrentGame)
		}
		return Response{Text: "Stream bot online | game: " + game + " | uptime: " + formatDuration(b.now().Sub(b.startedAt))}, true, nil
	case "game":
		state, err := b.store.Load()
		if err != nil {
			return Response{}, false, err
		}
		if state.CurrentGame == "" {
			return Response{Text: "Current game is not set."}, true, nil
		}
		return Response{Text: "Current game: " + DisplayGame(state.CurrentGame)}, true, nil
	case "setgame":
		if !msg.IsOwner {
			return Response{}, false, nil
		}
		game := NormalizeGame(strings.Join(args, " "))
		if game == "" {
			return Response{Text: "Usage: !setgame valorant, !setgame sky, or !setgame generic"}, true, nil
		}
		if _, err := b.store.Update(func(state State) State {
			state.CurrentGame = game
			return state
		}); err != nil {
			return Response{}, false, err
		}
		return Response{Text: "Current game set to " + DisplayGame(game) + "."}, true, nil
	case "valorant":
		return Response{Text: b.responses.Text("valorant")}, true, nil
	case "sky":
		return Response{Text: b.responses.Text("sky")}, true, nil
	case "generic":
		return Response{Text: b.responses.Text("generic")}, true, nil
	case "socials":
		return Response{Text: b.responses.Text("socials")}, true, nil
	case "specs":
		return Response{Text: b.responses.Text("specs")}, true, nil
	case "crosshair":
		return Response{Text: b.responses.Text("crosshair")}, true, nil
	case "isekai":
		return Response{Text: b.responses.Text("isekai")}, true, nil
	default:
		return Response{}, false, nil
	}
}

func (b *Bot) Announcement(ctx context.Context) (Response, bool, error) {
	if err := ctx.Err(); err != nil {
		return Response{}, false, err
	}
	state, err := b.store.Load()
	if err != nil {
		return Response{}, false, err
	}
	switch NormalizeGame(state.CurrentGame) {
	case "valorant":
		return Response{Text: b.responses.Text("welcome_valorant")}, true, nil
	case "sky":
		return Response{Text: b.responses.Text("welcome_sky")}, true, nil
	case "generic":
		return Response{Text: b.responses.Text("welcome_generic")}, true, nil
	default:
		return Response{}, false, nil
	}
}

func (b *Bot) SetGame(ctx context.Context, game string) (State, error) {
	if err := ctx.Err(); err != nil {
		return State{}, err
	}
	game = NormalizeGame(game)
	if game == "" {
		return b.store.Load()
	}
	return b.store.Update(func(state State) State {
		state.CurrentGame = game
		return state
	})
}

func isOwnerCommand(name string) bool {
	switch name {
	case "ping", "setgame", "status", "uptime":
		return true
	default:
		return false
	}
}

func (b *Bot) allow(name string) bool {
	if b.cooldown <= 0 {
		return true
	}

	now := b.now()
	b.mu.Lock()
	defer b.mu.Unlock()

	if last, ok := b.lastUsed[name]; ok && now.Sub(last) < b.cooldown {
		return false
	}
	b.lastUsed[name] = now
	return true
}

func parseCommand(text string) (name string, args []string, ok bool) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, CommandPrefix) {
		return "", nil, false
	}

	fields := strings.Fields(strings.TrimPrefix(text, CommandPrefix))
	if len(fields) == 0 {
		return "", nil, false
	}

	name = strings.ToLower(fields[0])
	switch name {
	case "skycotl", "skychildrenofthelight", "skychildren", "cotl":
		name = "sky"
	case "companion", "overlay", "skycompanion":
		name = "isekai"
	}
	return name, fields[1:], true
}

func DetectGame(title string, tags []string) string {
	text := strings.ToLower(title + " " + strings.Join(tags, " "))
	text = strings.NewReplacer("#", " ", "-", " ", "_", " ", ":", " ").Replace(text)
	text = strings.Join(strings.Fields(text), " ")
	switch {
	case strings.Contains(text, "valorant") || strings.Contains(text, " valo "):
		return "valorant"
	case strings.Contains(text, "sky children of the light") || strings.Contains(text, "sky cotl") || strings.Contains(text, "skycotl") || strings.Contains(text, " sky "):
		return "sky"
	default:
		return "generic"
	}
}

func NormalizeGame(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	raw = strings.Join(strings.Fields(raw), " ")
	switch raw {
	case "valorant", "valo":
		return "valorant"
	case "sky", "sky cotl", "skycotl", "sky children of the light":
		return "sky"
	case "generic", "general", "chat", "just chatting", "other":
		return "generic"
	default:
		return ""
	}
}

func DisplayGame(game string) string {
	switch NormalizeGame(game) {
	case "valorant":
		return "Valorant"
	case "sky":
		return "Sky: Children of the Light"
	case "generic":
		return "Generic"
	default:
		return "Unknown"
	}
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Truncate(time.Second)
	hours := int(d / time.Hour)
	d -= time.Duration(hours) * time.Hour
	minutes := int(d / time.Minute)
	d -= time.Duration(minutes) * time.Minute
	seconds := int(d / time.Second)

	if hours > 0 {
		return fmt.Sprintf("%dh%dm%ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}
