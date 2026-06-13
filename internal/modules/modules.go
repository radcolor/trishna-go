package modules

import (
	"fmt"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

type Module interface {
	Name() string
	Commands() []discord.ApplicationCommandCreate
	Register(router handler.Router)
}

type Registry struct {
	modules []Module
}

func NewRegistry(mods ...Module) (*Registry, error) {
	seenModules := make(map[string]struct{}, len(mods))
	seenCommands := make(map[string]string)

	for _, mod := range mods {
		if mod == nil {
			return nil, fmt.Errorf("module is nil")
		}

		moduleName := strings.TrimSpace(mod.Name())
		if moduleName == "" {
			return nil, fmt.Errorf("module name is required")
		}
		if _, exists := seenModules[moduleName]; exists {
			return nil, fmt.Errorf("duplicate module %q", moduleName)
		}
		seenModules[moduleName] = struct{}{}

		for _, command := range mod.Commands() {
			commandName := strings.TrimSpace(command.CommandName())
			if commandName == "" {
				return nil, fmt.Errorf("module %q has command with empty name", moduleName)
			}
			if owner, exists := seenCommands[commandName]; exists {
				return nil, fmt.Errorf("duplicate command %q in modules %q and %q", commandName, owner, moduleName)
			}
			seenCommands[commandName] = moduleName
		}
	}

	return &Registry{modules: append([]Module(nil), mods...)}, nil
}

func (r *Registry) Commands() []discord.ApplicationCommandCreate {
	var commands []discord.ApplicationCommandCreate
	for _, mod := range r.modules {
		commands = append(commands, mod.Commands()...)
	}
	return commands
}

func (r *Registry) Register(router handler.Router) {
	for _, mod := range r.modules {
		mod.Register(router)
	}
}
