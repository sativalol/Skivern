package fun

import (
	"math/rand"
	"skyvern/internal/manager"
	"skyvern/internal/plugins"
	"skyvern/internal/storage"
)

type FunPlugin struct{}

func init() {
	plugins.Register(&FunPlugin{})
}

func (p *FunPlugin) Name() string {
	return "fun"
}

func (p *FunPlugin) Init(db *storage.DB, mgr *manager.Manager) error {
	return nil
}

func (p *FunPlugin) Commands() []*manager.Command {
	return []*manager.Command{
		{
			Trigger:     "coinflip",
			Name:        "coinflip",
			Description: "Flip a coin",
			Category:    "fun",
			Execute: func(ctx *manager.CommandContext) error {
				s := []string{"Heads", "Tails"}
				r := s[rand.Intn(2)]
				return ctx.Reply("Result: " + r)
			},
		},
	}
}
