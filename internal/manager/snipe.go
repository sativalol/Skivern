package manager

import (
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

type DeletedMsg struct {
	Author    *discordgo.User
	Content   string
	ChannelID string
	Time      time.Time
}

type EditedMsg struct {
	Author    *discordgo.User
	Old       string
	New       string
	ChannelID string
	Time      time.Time
}

type DeletedReact struct {
	Author    *discordgo.User
	Emoji     *discordgo.Emoji
	ChannelID string
	Time      time.Time
}

var (
	DeletedMu sync.Mutex
	Deleted   = make(map[string][]DeletedMsg)

	EditedMu sync.Mutex
	Edited   = make(map[string][]EditedMsg)

	ReactMu sync.Mutex
	React   = make(map[string][]DeletedReact)
)

func AddDeleted(cid string, m DeletedMsg) {
	DeletedMu.Lock()
	defer DeletedMu.Unlock()
	Deleted[cid] = append([]DeletedMsg{m}, Deleted[cid]...)
	if len(Deleted[cid]) > 100 {
		Deleted[cid] = Deleted[cid][:100]
	}
}

func AddEdited(cid string, m EditedMsg) {
	EditedMu.Lock()
	defer EditedMu.Unlock()
	Edited[cid] = append([]EditedMsg{m}, Edited[cid]...)
	if len(Edited[cid]) > 100 {
		Edited[cid] = Edited[cid][:100]
	}
}

func AddReact(cid string, r DeletedReact) {
	ReactMu.Lock()
	defer ReactMu.Unlock()
	React[cid] = append([]DeletedReact{r}, React[cid]...)
	if len(React[cid]) > 100 {
		React[cid] = React[cid][:100]
	}
}

func ClearSnipe(cid string) {
	DeletedMu.Lock()
	delete(Deleted, cid)
	DeletedMu.Unlock()

	EditedMu.Lock()
	delete(Edited, cid)
	EditedMu.Unlock()

	ReactMu.Lock()
	delete(React, cid)
	ReactMu.Unlock()
}
