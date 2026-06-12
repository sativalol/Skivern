package music

import (
	"encoding/json"
	"fmt"
	"net/http"
	"skyvern/internal/config"
	"skyvern/internal/lavalink"
	"skyvern/internal/manager"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

var Play = &manager.Command{
	Trigger:     "play",
	Aliases:     []string{"p"},
	Name:        "play",
	Description: "Play a song from YouTube, Spotify, or SoundCloud",
	Category:    "music",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage: `.play <url_or_search_query>`")
		}

		l := ctx.Mgr.GetLavalink(ctx.ClientID)
		if l == nil {
			return ctx.Reply("[!] Music player not initialized.")
		}

		vcID := findUserVC(ctx.Session, ctx.Message.GuildID, ctx.Message.Author.ID)
		if vcID == "" {
			return ctx.Reply("[!] You must be in a voice channel to play music.")
		}

		err := lavalink.SendVoiceStateUpdate(ctx.Session, ctx.Message.GuildID, vcID, false, true)
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to join voice channel: %v", err))
		}

		q := strings.Join(ctx.Args, " ")
		tl, err := l.LoadTracks(q)
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to load tracks: %v", err))
		}

		tracks, playlistName, err := l.ParseLoadTracks(tl)
		if err != nil || len(tracks) == 0 {
			return ctx.Reply("[!] No tracks found.")
		}

		for i := range tracks {
			tracks[i].Requester = ctx.Message.Author.ID
		}
		p := l.GetPlayer(ctx.Message.GuildID)
		p.Add(tracks, ctx.Message.ChannelID)

		if playlistName != "" {
			return ctx.Reply(fmt.Sprintf("[+] Loaded playlist **%s** with %d tracks.", playlistName, len(tracks)))
		}
		if len(tracks) > 1 {
			return ctx.Reply(fmt.Sprintf("[+] Queued %d tracks.", len(tracks)))
		}
		return ctx.Reply(fmt.Sprintf("[+] Queued **%s**.", tracks[0].Info.Title))
	},
}

var Stop = &manager.Command{
	Trigger:     "stop",
	Aliases:     []string{"dc", "leave"},
	Name:        "stop",
	Description: "Stop music playback, clear queue, and leave voice channel",
	Category:    "music",
	Execute: func(ctx *manager.CommandContext) error {
		l := ctx.Mgr.GetLavalink(ctx.ClientID)
		if l == nil {
			return nil
		}
		p := l.GetPlayer(ctx.Message.GuildID)
		_ = p.Stop()
		l.RemovePlayer(ctx.Message.GuildID)
		return ctx.Reply("[+] Stopped playback and left voice channel.")
	},
}

var Pause = &manager.Command{
	Trigger:     "pause",
	Name:        "pause",
	Description: "Pause music playback",
	Category:    "music",
	Execute: func(ctx *manager.CommandContext) error {
		l := ctx.Mgr.GetLavalink(ctx.ClientID)
		if l == nil {
			return nil
		}
		p := l.GetPlayer(ctx.Message.GuildID)
		if err := p.Pause(true); err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to pause: %v", err))
		}
		return ctx.Reply("[+] Paused.")
	},
}

var Resume = &manager.Command{
	Trigger:     "resume",
	Aliases:     []string{"unpause"},
	Name:        "resume",
	Description: "Resume music playback",
	Category:    "music",
	Execute: func(ctx *manager.CommandContext) error {
		l := ctx.Mgr.GetLavalink(ctx.ClientID)
		if l == nil {
			return nil
		}
		p := l.GetPlayer(ctx.Message.GuildID)
		if err := p.Pause(false); err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to resume: %v", err))
		}
		return ctx.Reply("[+] Resumed.")
	},
}

var Skip = &manager.Command{
	Trigger:     "skip",
	Aliases:     []string{"s", "next"},
	Name:        "skip",
	Description: "Skip the current song",
	Category:    "music",
	Execute: func(ctx *manager.CommandContext) error {
		l := ctx.Mgr.GetLavalink(ctx.ClientID)
		if l == nil {
			return nil
		}
		p := l.GetPlayer(ctx.Message.GuildID)
		if err := p.Skip(); err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to skip: %v", err))
		}
		return ctx.Reply("[+] Skipped.")
	},
}

var Queue = &manager.Command{
	Trigger:     "queue",
	Aliases:     []string{"q"},
	Name:        "queue",
	Description: "Display the current track queue",
	Category:    "music",
	Execute: func(ctx *manager.CommandContext) error {
		l := ctx.Mgr.GetLavalink(ctx.ClientID)
		if l == nil {
			return nil
		}
		p := l.GetPlayer(ctx.Message.GuildID)
		q, cur := p.GetQueue()
		if len(q) == 0 {
			return ctx.Reply("[*] Queue is empty.")
		}

		var sb strings.Builder
		if cur >= 0 && cur < len(q) {
			current := q[cur]
			reqMention := "Unknown"
			if current.Requester != "" {
				reqMention = fmt.Sprintf("<@%s>", current.Requester)
			}
			sb.WriteString(fmt.Sprintf("**Now Playing:**\n**[%s](%s)**\n%s • %s • %s\n\n",
				current.Info.Title, current.Info.URI, current.Info.Author, formatDur(current.Info.Length), reqMention))
		}

		upNext := q[cur+1:]
		if len(upNext) > 0 {
			sb.WriteString("**Up Next:**\n")
			limit := len(upNext)
			if limit > 10 {
				limit = 10
			}
			for i := 0; i < limit; i++ {
				t := upNext[i]
				reqMention := "Unknown"
				if t.Requester != "" {
					reqMention = fmt.Sprintf("<@%s>", t.Requester)
				}
				sb.WriteString(fmt.Sprintf("`%d.` **[%s](%s)**\n%s • %s • %s\n",
					i+1, t.Info.Title, t.Info.URI, t.Info.Author, formatDur(t.Info.Length), reqMention))
			}
			if len(upNext) > 10 {
				sb.WriteString(fmt.Sprintf("\n*...and %d more track(s)*\n", len(upNext)-10))
			}
		} else if cur < 0 || cur >= len(q) {
			sb.WriteString("The queue is currently empty.")
		}

		remainingTracks := len(upNext)
		if cur >= 0 && cur < len(q) {
			remainingTracks++
		}
		sb.WriteString(fmt.Sprintf("\n**Loop:** %s | **Total:** %d tracks", p.LoopMode(), remainingTracks))

		emb := config.Build(ctx.Cfg, config.EmbedOpt{
			Title:       "Queue",
			Description: sb.String(),
		})
		return ctx.Respond(emb)
	},
}

var NP = &manager.Command{
	Trigger:     "np",
	Aliases:     []string{"nowplaying"},
	Name:        "np",
	Description: "Show details about the currently playing song",
	Category:    "music",
	Execute: func(ctx *manager.CommandContext) error {
		l := ctx.Mgr.GetLavalink(ctx.ClientID)
		if l == nil {
			return nil
		}
		p := l.GetPlayer(ctx.Message.GuildID)
		t, _, ok := p.NowPlaying()
		if !ok {
			return ctx.Reply("[*] Nothing is currently playing.")
		}

		pos := int64(0)
		if playerInfo, err := getLavalinkPlayer(l, ctx.Message.GuildID); err == nil {
			pos = playerInfo.Position
		}

		isPausedStr := "Playing"
		if p.Paused() {
			isPausedStr = "Paused"
		}
		reqMention := "Unknown"
		if t.Requester != "" {
			reqMention = fmt.Sprintf("<@%s>", t.Requester)
		}
		desc := fmt.Sprintf("**[%s](%s)**\n\n**Status:** %s\n**Duration:** %s / %s\n**Requested By:** %s\n**Loop:** %s",
			t.Info.Title, t.Info.URI, isPausedStr, formatDur(pos), formatDur(t.Info.Length), reqMention, p.LoopMode())

		emb := config.Build(ctx.Cfg, config.EmbedOpt{
			Title:       "Now Playing",
			Description: desc,
		})
		return ctx.Respond(emb)
	},
}

var Volume = &manager.Command{
	Trigger:     "volume",
	Aliases:     []string{"vol"},
	Name:        "volume",
	Description: "Set player volume level",
	Category:    "music",
	Execute: func(ctx *manager.CommandContext) error {
		l := ctx.Mgr.GetLavalink(ctx.ClientID)
		if l == nil {
			return nil
		}
		p := l.GetPlayer(ctx.Message.GuildID)
		if len(ctx.Args) == 0 {
			return ctx.Reply(fmt.Sprintf("[*] Current volume is **%d%%**.", p.Vol()))
		}
		v, err := strconv.Atoi(ctx.Args[0])
		if err != nil || v < 0 || v > 150 {
			return ctx.Reply("[!] Volume must be a number between 0 and 150.")
		}
		if err := p.Volume(v); err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to set volume: %v", err))
		}
		return ctx.Reply(fmt.Sprintf("[+] Set volume to **%d%%**.", v))
	},
}

var Seek = &manager.Command{
	Trigger:     "seek",
	Aliases:     []string{"ff", "fastforward"},
	Name:        "seek",
	Description: "Seek to a timestamp (e.g. 1:30 or 90)",
	Category:    "music",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage: `.seek <1:30|90>`")
		}
		l := ctx.Mgr.GetLavalink(ctx.ClientID)
		if l == nil {
			return nil
		}
		p := l.GetPlayer(ctx.Message.GuildID)

		input := ctx.Args[0]
		var ms int64
		if strings.Contains(input, ":") {
			parts := strings.Split(input, ":")
			if len(parts) == 2 {
				m, _ := strconv.Atoi(parts[0])
				s, _ := strconv.Atoi(parts[1])
				ms = int64(m*60+s) * 1000
			} else if len(parts) == 3 {
				h, _ := strconv.Atoi(parts[0])
				m, _ := strconv.Atoi(parts[1])
				s, _ := strconv.Atoi(parts[2])
				ms = int64(h*3600+m*60+s) * 1000
			}
		} else {
			s, _ := strconv.Atoi(input)
			ms = int64(s) * 1000
		}

		if err := p.Seek(ms); err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Seek failed: %v", err))
		}
		return ctx.Reply(fmt.Sprintf("[+] Seeked to **%s**.", formatDur(ms)))
	},
}

var Loop = &manager.Command{
	Trigger:     "loop",
	Aliases:     []string{"repeat"},
	Name:        "loop",
	Description: "Set queue or track loop mode",
	Category:    "music",
	Execute: func(ctx *manager.CommandContext) error {
		l := ctx.Mgr.GetLavalink(ctx.ClientID)
		if l == nil {
			return nil
		}
		p := l.GetPlayer(ctx.Message.GuildID)

		mode := "off"
		if len(ctx.Args) > 0 {
			mode = strings.ToLower(ctx.Args[0])
		} else {
			switch p.LoopMode() {
			case "off":
				mode = "track"
			case "track":
				mode = "queue"
			default:
				mode = "off"
			}
		}

		if mode != "off" && mode != "track" && mode != "queue" {
			return ctx.Reply("[!] Invalid mode. Choose from: off, track, queue.")
		}

		p.SetLoop(mode)
		return ctx.Reply(fmt.Sprintf("[+] Loop mode set to **%s**.", mode))
	},
}

var Shuffle = &manager.Command{
	Trigger:     "shuffle",
	Aliases:     []string{"shuf"},
	Name:        "shuffle",
	Description: "Shuffle the music queue",
	Category:    "music",
	Execute: func(ctx *manager.CommandContext) error {
		l := ctx.Mgr.GetLavalink(ctx.ClientID)
		if l == nil {
			return nil
		}
		p := l.GetPlayer(ctx.Message.GuildID)
		p.Shuffle()
		return ctx.Reply("[+] Shuffled the queue.")
	},
}

var Clear = &manager.Command{
	Trigger:     "clear",
	Aliases:     []string{"clearqueue"},
	Name:        "clear",
	Description: "Clear the music queue",
	Category:    "music",
	Execute: func(ctx *manager.CommandContext) error {
		l := ctx.Mgr.GetLavalink(ctx.ClientID)
		if l == nil {
			return nil
		}
		p := l.GetPlayer(ctx.Message.GuildID)
		p.Clear()
		return ctx.Reply("[+] Cleared the queue.")
	},
}

func findUserVC(s *discordgo.Session, gid, uid string) string {
	g, err := s.State.Guild(gid)
	if err != nil {
		g, err = s.Guild(gid)
		if err != nil {
			return ""
		}
	}
	for _, vs := range g.VoiceStates {
		if vs.UserID == uid {
			return vs.ChannelID
		}
	}
	return ""
}

func formatDur(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

func makeProgressBar(current, total int64, width int) string {
	if total <= 0 {
		return "[" + strings.Repeat("-", width) + "]"
	}
	progress := float64(current) / float64(total)
	pos := int(progress * float64(width))
	if pos < 0 {
		pos = 0
	}
	if pos > width {
		pos = width
	}
	bar := strings.Repeat("▬", pos) + "🔘" + strings.Repeat("▬", width-pos)
	return bar
}

type lavalinkPlayer struct {
	Position int64 `json:"position"`
}

func getLavalinkPlayer(l *lavalink.Client, gid string) (*lavalinkPlayer, error) {
	sess := l.SessID()
	if sess == "" {
		return nil, fmt.Errorf("no session")
	}
	u := fmt.Sprintf("http://%s/v4/sessions/%s/players/%s", l.Host(), sess, gid)
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("Authorization", l.Pwd())

	resp, err := lavalink.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status: %d", resp.StatusCode)
	}

	var res lavalinkPlayer
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return &res, nil
}
