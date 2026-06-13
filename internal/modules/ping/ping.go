package ping

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

const CommandName = "ping"

type Module struct{}

func New() Module {
	return Module{}
}

func (Module) Name() string {
	return CommandName
}

func (Module) Commands() []discord.ApplicationCommandCreate {
	return []discord.ApplicationCommandCreate{
		discord.SlashCommandCreate{
			Name:        CommandName,
			Description: "Check whether Trishna is online.",
		},
	}
}

func (m Module) Register(router handler.Router) {
	router.Command("/"+CommandName, m.HandleInteraction)
}

func (Module) HandleInteraction(event *handler.CommandEvent) error {
	return event.CreateMessage(Response())
}

func Response() discord.MessageCreate {
	return discord.MessageCreate{Content: "jihyooo ❤️"}
}
