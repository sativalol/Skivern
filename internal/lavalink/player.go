package lavalink

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

type Player struct {
	client  *Client
	guildID string
	chanID  string
	queue   []Track
	cur     int
	vol     int
	paused  bool
	loop    string
	mu      sync.Mutex
}

func (p *Player) Add(tracks []Track, textChan string) {
	p.mu.Lock()
	p.chanID = textChan
	start := len(p.queue) == 0
	p.queue = append(p.queue, tracks...)
	p.mu.Unlock()

	if start {
		p.PlayIndex(0)
	}
}

func (p *Player) PlayIndex(idx int) {
	p.mu.Lock()
	if idx < 0 || idx >= len(p.queue) {
		p.mu.Unlock()
		return
	}
	p.cur = idx
	p.paused = false
	t := p.queue[p.cur]
	p.mu.Unlock()

	payload := map[string]any{
		"encodedTrack": t.Encoded,
		"paused":       false,
	}
	_ = p.client.UpdatePlayer(p.guildID, payload)
}

func (p *Player) PlayNext() {
	p.mu.Lock()
	if len(p.queue) == 0 {
		p.mu.Unlock()
		return
	}
	next := p.cur
	switch p.loop {
	case "track":
		// keep current index
	case "queue":
		next = (p.cur + 1) % len(p.queue)
	default:
		next = p.cur + 1
	}

	if next < 0 || next >= len(p.queue) {
		p.queue = nil
		p.cur = 0
		cid := p.chanID
		p.mu.Unlock()

		_ = p.client.DestroyPlayer(p.guildID)
		if cid != "" && p.client.Session != nil {
			_, _ = p.client.Session.ChannelMessageSend(cid, "Queue finished.")
		}
		return
	}

	p.cur = next
	p.mu.Unlock()
	p.PlayIndex(next)
}

func (p *Player) Skip() error {
	p.mu.Lock()
	if p.cur+1 >= len(p.queue) && p.loop != "queue" {
		p.mu.Unlock()
		return p.Stop()
	}
	p.mu.Unlock()
	p.PlayNext()
	return nil
}

func (p *Player) Stop() error {
	p.mu.Lock()
	p.queue = nil
	p.cur = 0
	p.mu.Unlock()

	_ = p.client.DestroyPlayer(p.guildID)
	if p.client.Session != nil {
		_ = SendVoiceStateUpdate(p.client.Session, p.guildID, "", false, false)
	}
	return nil
}

func (p *Player) Pause(paused bool) error {
	p.mu.Lock()
	p.paused = paused
	p.mu.Unlock()

	payload := map[string]any{
		"paused": paused,
	}
	return p.client.UpdatePlayer(p.guildID, payload)
}

func (p *Player) Volume(v int) error {
	if v < 0 {
		v = 0
	}
	if v > 150 {
		v = 150
	}
	p.mu.Lock()
	p.vol = v
	p.mu.Unlock()

	payload := map[string]any{
		"volume": v,
	}
	return p.client.UpdatePlayer(p.guildID, payload)
}

func (p *Player) Seek(ms int64) error {
	payload := map[string]any{
		"position": ms,
	}
	return p.client.UpdatePlayer(p.guildID, payload)
}

func (p *Player) Shuffle() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.queue) <= p.cur+1 {
		return
	}
	sub := p.queue[p.cur+1:]
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(sub), func(i, j int) {
		sub[i], sub[j] = sub[j], sub[i]
	})
}

func (p *Player) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.queue) == 0 {
		return
	}
	p.queue = []Track{p.queue[p.cur]}
	p.cur = 0
}

func (p *Player) SetLoop(mode string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.loop = mode
}

func (p *Player) NowPlaying() (Track, int64, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cur < 0 || p.cur >= len(p.queue) {
		return Track{}, 0, false
	}
	return p.queue[p.cur], int64(p.cur), true
}

func (p *Player) GetQueue() ([]Track, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.queue, p.cur
}

func (p *Player) LoopMode() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.loop == "" {
		return "off"
	}
	return p.loop
}

func (p *Player) Paused() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.paused
}

func (p *Player) AnnounceStart() {
	p.mu.Lock()
	cid := p.chanID
	if cid == "" || p.client.Session == nil || p.cur < 0 || p.cur >= len(p.queue) {
		p.mu.Unlock()
		return
	}
	t := p.queue[p.cur]
	isPaused := p.paused
	loopMode := p.loop
	if loopMode == "" {
		loopMode = "off"
	}
	p.mu.Unlock()

	isPausedStr := "Playing"
	if isPaused {
		isPausedStr = "Paused"
	}
	reqMention := "Unknown"
	if t.Requester != "" {
		reqMention = fmt.Sprintf("<@%s>", t.Requester)
	}

	emb := &discordgo.MessageEmbed{
		Title: "Now Playing",
		Description: fmt.Sprintf("**[%s](%s)**\n\n**Status:** %s\n**Duration:** 0:00 / %s\n**Requested By:** %s\n**Loop:** %s",
			t.Info.Title, t.Info.URI, isPausedStr, formatDur(t.Info.Length), reqMention, loopMode),
		Color: 0x00ff00,
	}
	_, _ = p.client.Session.ChannelMessageSendEmbed(cid, emb)
}

func (p *Player) Vol() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.vol
}

func formatDur(ms int64) string {
	s := (ms / 1000) % 60
	m := (ms / (1000 * 60)) % 60
	h := (ms / (1000 * 60 * 60)) % 24
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
