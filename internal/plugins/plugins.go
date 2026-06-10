package plugins

import (
	"skyvern/internal/manager"
	"skyvern/internal/storage"
)

type Plugin interface {
	Name() string
	Init(db *storage.DB, mgr *manager.Manager) error
	Commands() []*manager.Command
}

var reg []Plugin

func Register(p Plugin) {
	reg = append(reg, p)
}

func Loaded() []Plugin {
	return reg
}
