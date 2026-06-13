package modules

import (
	"strings"
	"testing"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

func TestRegistryRejectsDuplicateModuleNames(t *testing.T) {
	_, err := NewRegistry(stubModule{name: "ping"}, stubModule{name: "ping"})
	if err == nil {
		t.Fatal("expected duplicate module error")
	}
	if !strings.Contains(err.Error(), "duplicate module") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegistryRejectsDuplicateCommandNames(t *testing.T) {
	_, err := NewRegistry(
		stubModule{name: "one", commands: []discord.ApplicationCommandCreate{slash("ping")}},
		stubModule{name: "two", commands: []discord.ApplicationCommandCreate{slash("ping")}},
	)
	if err == nil {
		t.Fatal("expected duplicate command error")
	}
	if !strings.Contains(err.Error(), "duplicate command") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegistryCommandsFlattensModules(t *testing.T) {
	registry, err := NewRegistry(
		stubModule{name: "one", commands: []discord.ApplicationCommandCreate{slash("one")}},
		stubModule{name: "two", commands: []discord.ApplicationCommandCreate{slash("two")}},
	)
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	commands := registry.Commands()
	if len(commands) != 2 {
		t.Fatalf("commands len = %d", len(commands))
	}
	if commands[0].CommandName() != "one" || commands[1].CommandName() != "two" {
		t.Fatalf("commands = %q, %q", commands[0].CommandName(), commands[1].CommandName())
	}
}

type stubModule struct {
	name     string
	commands []discord.ApplicationCommandCreate
}

func (m stubModule) Name() string {
	return m.name
}

func (m stubModule) Commands() []discord.ApplicationCommandCreate {
	return m.commands
}

func (m stubModule) Register(handler.Router) {}

func slash(name string) discord.ApplicationCommandCreate {
	return discord.SlashCommandCreate{Name: name, Description: name}
}
