package utility

import (
	"fmt"
	"skyvern/internal/manager"
	"time"
)

var start = time.Now()

var Uptime = &manager.Command{
	Trigger:     "uptime",
	Name:        "uptime",
	Description: "Shows how long the engine has been running",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		diff := time.Since(start).Round(time.Second)
		return ctx.Reply(fmt.Sprintf("Engine uptime: %s", diff))
	},
}
