package general

import (
	"skyvern/internal/manager"
)

var Ping = &manager.Command{
	Trigger:     "ping",
	Name:        "ping",
	Description: "Check bot responsiveness",
	Category:    "general",
	Execute: func(ctx *manager.CommandContext) error {
		return ctx.Reply("pong")
	},
}
