package lavalink

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gorilla/websocket"
)

var HTTPClient = &http.Client{
	Timeout: 5 * time.Second,
}

type Track struct {
	Encoded   string `json:"encoded"`
	Requester string `json:"requester,omitempty"`
	Info      struct {
		Identifier string `json:"identifier"`
		Title      string `json:"title"`
		Author     string `json:"author"`
		Length     int64  `json:"length"`
		Position   int64  `json:"position"`
		URI        string `json:"uri"`
		Source     string `json:"sourceName"`
	} `json:"info"`
}

type TrackList struct {
	LoadType string `json:"loadType"`
	Data     any    `json:"data"`
}

type PlaylistData struct {
	Info struct {
		Name string `json:"name"`
	} `json:"info"`
	Tracks []Track `json:"tracks"`
}

type VoiceState struct {
	Token     string `json:"token"`
	Endpoint  string `json:"endpoint"`
	SessionID string `json:"sessionId"`
	ChannelID string `json:"channelId"`
}

type node struct {
	host string
	pwd  string
}

var publicNodes = []node{
	{"lavalink.jirayu.net:13592", "youshallnotpass"},
	{"lavalink.triniumhost.com:4333", "free"},
	{"lavalink.triniumhost.com:2333", "kirito"},
	{"lavalinkv4.serenetia.com:80", "https://seretia.link/discord"},
	{"sg1-nodelink.nyxbot.app:3000", "nyxbot.app/support"},
	{"sg2-nodelink.nyxbot.app:3000", "nyxbot.app/support"},
	{"n3.nexcloud.in:2026", "nexcloud"},
}

type Client struct {
	host         string
	pwd          string
	userID       string
	sessID       string
	ws           *websocket.Conn
	mu           sync.Mutex
	players      map[string]*Player
	vStates      map[string]*VoiceState
	vStatesMu    sync.Mutex
	closeChan    chan struct{}
	Session      *discordgo.Session
	nodes        []node
	curNode      int
	logs         []string
	logsMu       sync.Mutex
}

type NodeInfo struct {
	Host   string
	Active bool
}

func (c *Client) log(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	ts := time.Now().Format("15:04:05")
	fullMsg := fmt.Sprintf("[%s] %s", ts, strings.TrimRight(msg, "\n"))

	c.logsMu.Lock()
	c.logs = append(c.logs, fullMsg)
	if len(c.logs) > 100 {
		c.logs = c.logs[1:]
	}
	c.logsMu.Unlock()

	_, _ = fmt.Fprintln(os.Stderr, fullMsg)
}

func (c *Client) GetLogs() []string {
	c.logsMu.Lock()
	defer c.logsMu.Unlock()
	out := make([]string, len(c.logs))
	copy(out, c.logs)
	return out
}

func (c *Client) GetNodes() []NodeInfo {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []NodeInfo
	for i, n := range c.nodes {
		out = append(out, NodeInfo{
			Host:   n.host,
			Active: i == c.curNode,
		})
	}
	return out
}


func NewClient(host, pwd, userID string, s *discordgo.Session) *Client {
	c := &Client{
		host:      host,
		pwd:       pwd,
		userID:    userID,
		players:   make(map[string]*Player),
		vStates:   make(map[string]*VoiceState),
		closeChan: make(chan struct{}),
		Session:   s,
	}
	c.nodes = append(c.nodes, node{host: host, pwd: pwd})
	for _, n := range publicNodes {
		if n.host != host {
			c.nodes = append(c.nodes, n)
		}
	}
	go c.connect()
	go c.monitorPreferredNode()
	return c
}

func (c *Client) Close() {
	close(c.closeChan)
	c.mu.Lock()
	if c.ws != nil {
		_ = c.ws.Close()
	}
	c.mu.Unlock()
}

func (c *Client) cycleNode(failedHost string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.nodes) <= 1 {
		return
	}
	if failedHost != c.host {
		return
	}
	c.curNode = (c.curNode + 1) % len(c.nodes)
	c.host = c.nodes[c.curNode].host
	c.pwd = c.nodes[c.curNode].pwd
	c.sessID = ""
	if c.ws != nil {
		_ = c.ws.Close()
		c.ws = nil
	}
	c.log("Lavalink Connection failed/lost on %s, switching to fallback: %s", failedHost, c.host)
}

func (c *Client) connect() {
	for {
		c.mu.Lock()
		activeHost := c.host
		activePwd := c.pwd
		preferredHost := c.nodes[0].host
		preferredPwd := c.nodes[0].pwd
		currentCurNode := c.curNode
		c.mu.Unlock()

		// If on fallback node, check if preferred node (index 0) has come online
		if currentCurNode != 0 {
			u := url.URL{Scheme: "ws", Host: preferredHost, Path: "/v4/websocket"}
			headers := http.Header{}
			headers.Set("Authorization", preferredPwd)
			headers.Set("User-Id", c.userID)
			headers.Set("Client-Name", "Skyvern")

			conn, _, err := websocket.DefaultDialer.Dial(u.String(), headers)
			if err == nil {
				c.mu.Lock()
				c.curNode = 0
				c.host = preferredHost
				c.pwd = preferredPwd
				activeHost = preferredHost
				activePwd = preferredPwd
				c.ws = conn
				c.mu.Unlock()

				c.log("Successfully reconnected to preferred Lavalink node: %s", preferredHost)
				for {
					_, msg, err := conn.ReadMessage()
					if err != nil {
						break
					}
					c.handleWSMessage(msg)
				}

				c.mu.Lock()
				c.ws = nil
				c.mu.Unlock()

				select {
				case <-c.closeChan:
					return
				default:
					time.Sleep(2 * time.Second)
					continue
				}
			}
		}

		u := url.URL{Scheme: "ws", Host: activeHost, Path: "/v4/websocket"}
		headers := http.Header{}
		headers.Set("Authorization", activePwd)
		headers.Set("User-Id", c.userID)
		headers.Set("Client-Name", "Skyvern")

		conn, _, err := websocket.DefaultDialer.Dial(u.String(), headers)
		if err != nil {
			c.cycleNode(activeHost)
			time.Sleep(2 * time.Second)
			continue
		}

		c.log("Connected websocket to Lavalink node: %s", activeHost)

		c.mu.Lock()
		c.ws = conn
		c.mu.Unlock()

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				break
			}
			c.handleWSMessage(msg)
		}

		c.mu.Lock()
		c.ws = nil
		c.mu.Unlock()

		select {
		case <-c.closeChan:
			return
		default:
			c.cycleNode(activeHost)
			time.Sleep(2 * time.Second)
		}
	}
}

type rawMsg struct {
	Op        string `json:"op"`
	SessionID string `json:"sessionId"`
	Type      string `json:"type"`
	GuildID   string `json:"guildId"`
	Reason    string `json:"reason"`
}

func (c *Client) handleWSMessage(msg []byte) {
	c.log("[Lavalink WS] %s", string(msg))
	var m rawMsg
	if err := json.Unmarshal(msg, &m); err != nil {
		return
	}
	switch m.Op {
	case "ready":
		c.mu.Lock()
		c.sessID = m.SessionID
		c.mu.Unlock()
	case "event":
		p := c.GetPlayer(m.GuildID)
		if p == nil {
			return
		}
		if m.Type == "TrackStartEvent" {
			p.AnnounceStart()
		} else if m.Type == "TrackEndEvent" {
			if m.Reason == "finished" || m.Reason == "loadFailed" {
				p.PlayNext()
			}
		}
	}
}

func (c *Client) LoadTracks(q string) (*TrackList, error) {
	if !strings.HasPrefix(q, "http://") && !strings.HasPrefix(q, "https://") {
		q = "ytsearch:" + q
	}

	var lastErr error
	c.mu.Lock()
	limit := len(c.nodes)
	c.mu.Unlock()
	if limit < 3 {
		limit = 3
	}
	if limit > 6 {
		limit = 6
	}

	for i := 0; i < limit; i++ {
		c.mu.Lock()
		activeHost := c.host
		activePwd := c.pwd
		c.mu.Unlock()

		u := fmt.Sprintf("http://%s/v4/loadtracks?identifier=%s", activeHost, url.QueryEscape(q))
		req, err := http.NewRequest("GET", u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", activePwd)

		resp, err := HTTPClient.Do(req)
		if err == nil {
			if resp.StatusCode == http.StatusOK {
				var res TrackList
				err = json.NewDecoder(resp.Body).Decode(&res)
				resp.Body.Close()
				if err == nil {
					return &res, nil
				}
			} else {
				resp.Body.Close()
				err = fmt.Errorf("status: %d", resp.StatusCode)
			}
		}

		lastErr = err
		c.log("[Lavalink] LoadTracks failed on %s: %v. Retrying on next node...", activeHost, err)
		c.cycleNode(activeHost)
		time.Sleep(1 * time.Second)
	}

	return nil, fmt.Errorf("load tracks failed: %w", lastErr)
}

func (c *Client) ParseLoadTracks(tl *TrackList) ([]Track, string, error) {
	b, err := json.Marshal(tl.Data)
	if err != nil {
		return nil, "", err
	}

	switch tl.LoadType {
	case "track":
		var t Track
		if err := json.Unmarshal(b, &t); err != nil {
			return nil, "", err
		}
		return []Track{t}, "", nil
	case "playlist":
		var pd PlaylistData
		if err := json.Unmarshal(b, &pd); err != nil {
			return nil, "", err
		}
		return pd.Tracks, pd.Info.Name, nil
	case "search":
		var ts []Track
		if err := json.Unmarshal(b, &ts); err != nil {
			return nil, "", err
		}
		if len(ts) > 0 {
			return ts[:1], "", nil
		}
		return nil, "", nil
	}
	return nil, "", fmt.Errorf("unknown loadType: %s", tl.LoadType)
}

func (c *Client) UpdatePlayer(guildID string, body map[string]any) error {
	var lastErr error
	c.mu.Lock()
	limit := len(c.nodes)
	c.mu.Unlock()
	if limit < 3 {
		limit = 3
	}
	if limit > 6 {
		limit = 6
	}

	for i := 0; i < limit; i++ {
		c.mu.Lock()
		activeHost := c.host
		activePwd := c.pwd
		sess := c.sessID
		c.mu.Unlock()

		if sess == "" {
			time.Sleep(1 * time.Second)
			lastErr = fmt.Errorf("no active session")
			continue
		}

		u := fmt.Sprintf("http://%s/v4/sessions/%s/players/%s", activeHost, sess, guildID)
		buf := new(bytes.Buffer)
		_ = json.NewEncoder(buf).Encode(body)

		req, err := http.NewRequest("PATCH", u, buf)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", activePwd)
		req.Header.Set("Content-Type", "application/json")

		resp, err := HTTPClient.Do(req)
		if err == nil {
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				resp.Body.Close()
				return nil
			}
			resp.Body.Close()
			err = fmt.Errorf("status: %d", resp.StatusCode)
		}

		lastErr = err
		c.log("[Lavalink] UpdatePlayer failed on %s: %v. Retrying on next node...", activeHost, err)
		c.cycleNode(activeHost)
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("update player failed: %w", lastErr)
}

func (c *Client) DestroyPlayer(guildID string) error {
	var lastErr error
	c.mu.Lock()
	limit := len(c.nodes)
	c.mu.Unlock()
	if limit < 3 {
		limit = 3
	}
	if limit > 6 {
		limit = 6
	}

	for i := 0; i < limit; i++ {
		c.mu.Lock()
		activeHost := c.host
		activePwd := c.pwd
		sess := c.sessID
		c.mu.Unlock()

		if sess == "" {
			time.Sleep(1 * time.Second)
			lastErr = fmt.Errorf("no active session")
			continue
		}

		u := fmt.Sprintf("http://%s/v4/sessions/%s/players/%s", activeHost, sess, guildID)
		req, _ := http.NewRequest("DELETE", u, nil)
		req.Header.Set("Authorization", activePwd)

		resp, err := HTTPClient.Do(req)
		if err == nil {
			resp.Body.Close()
			return nil
		}

		lastErr = err
		c.log("[Lavalink] DestroyPlayer failed on %s: %v. Retrying on next node...", activeHost, err)
		c.cycleNode(activeHost)
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("destroy player failed: %w", lastErr)
}

func (c *Client) HandleVoiceState(guildID, sessionID, channelID string) {
	c.log("HandleVoiceState: guildID=%s, sessionID=%s, channelID=%s", guildID, sessionID, channelID)
	c.vStatesMu.Lock()
	vs, ok := c.vStates[guildID]
	if !ok {
		vs = &VoiceState{}
		c.vStates[guildID] = vs
	}
	vs.SessionID = sessionID
	if channelID != "" {
		vs.ChannelID = channelID
	}
	ready := vs.Token != "" && vs.Endpoint != "" && vs.SessionID != "" && vs.ChannelID != ""
	c.vStatesMu.Unlock()

	if ready {
		c.sendVoiceUpdate(guildID, vs)
	}
}

func (c *Client) HandleVoiceServer(guildID, token, endpoint string) {
	c.log("HandleVoiceServer: guildID=%s, token=..., endpoint=%s", guildID, endpoint)
	c.vStatesMu.Lock()
	vs, ok := c.vStates[guildID]
	if !ok {
		vs = &VoiceState{}
		c.vStates[guildID] = vs
	}
	vs.Token = token
	vs.Endpoint = endpoint
	ready := vs.Token != "" && vs.Endpoint != "" && vs.SessionID != "" && vs.ChannelID != ""
	c.vStatesMu.Unlock()

	if ready {
		c.sendVoiceUpdate(guildID, vs)
	}
}

func (c *Client) sendVoiceUpdate(guildID string, vs *VoiceState) {
	c.log("sendVoiceUpdate: sending voice payload for guildID=%s, token=..., endpoint=%s, session=%s, channel=%s", guildID, vs.Endpoint, vs.SessionID, vs.ChannelID)
	payload := map[string]any{
		"voice": map[string]any{
			"token":     vs.Token,
			"endpoint":  vs.Endpoint,
			"sessionId": vs.SessionID,
			"channelId": vs.ChannelID,
		},
	}
	err := c.UpdatePlayer(guildID, payload)
	if err != nil {
		c.log("sendVoiceUpdate failed: %v", err)
	} else {
		c.log("sendVoiceUpdate succeeded")
	}
}

func (c *Client) GetPlayer(guildID string) *Player {
	c.mu.Lock()
	defer c.mu.Unlock()
	p, ok := c.players[guildID]
	if !ok {
		p = &Player{
			client:  c,
			guildID: guildID,
			vol:     100,
		}
		c.players[guildID] = p
	}
	return p
}

func (c *Client) RemovePlayer(guildID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.players, guildID)
}

func (c *Client) Host() string {
	return c.host
}

func (c *Client) Pwd() string {
	return c.pwd
}

func (c *Client) SessID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sessID
}

func SendVoiceStateUpdate(s *discordgo.Session, guildID, channelID string, mute, deaf bool) error {
	var cid *string
	if channelID != "" {
		cid = &channelID
	}
	payload := struct {
		Op int `json:"op"`
		D  struct {
			GuildID   string  `json:"guild_id"`
			ChannelID *string `json:"channel_id"`
			SelfMute  bool    `json:"self_mute"`
			SelfDeaf  bool    `json:"self_deaf"`
		} `json:"d"`
	}{}
	payload.Op = 4
	payload.D.GuildID = guildID
	payload.D.ChannelID = cid
	payload.D.SelfMute = mute
	payload.D.SelfDeaf = deaf

	return s.GatewayWriteStruct(payload)
}

var lavalinkCmd *exec.Cmd

func StartServer(resolvePath func(string) string) {
	if lavalinkCmd != nil {
		return
	}
	jarPath := resolvePath("lavalink/Lavalink.jar")
	if _, err := os.Stat(jarPath); os.IsNotExist(err) {
		return
	}
	cmd := exec.Command("java", "-jar", "Lavalink.jar")
	cmd.Dir = filepath.Dir(jarPath)
	logFile, err := os.OpenFile(resolvePath("lavalink.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err == nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}
	if err := cmd.Start(); err == nil {
		lavalinkCmd = cmd
	}
}

func StopServer() {
	if lavalinkCmd != nil && lavalinkCmd.Process != nil {
		_ = lavalinkCmd.Process.Kill()
		lavalinkCmd = nil
	}
}

func (c *Client) hasActivePlayers() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, p := range c.players {
		p.mu.Lock()
		playing := len(p.queue) > 0 && !p.paused
		p.mu.Unlock()
		if playing {
			return true
		}
	}
	return false
}

func (c *Client) monitorPreferredNode() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.closeChan:
			return
		case <-ticker.C:
			c.mu.Lock()
			currentCurNode := c.curNode
			preferredHost := c.nodes[0].host
			c.mu.Unlock()

			if currentCurNode == 0 {
				continue
			}

			if c.hasActivePlayers() {
				continue
			}

			c.mu.Lock()
			preferredPwd := c.nodes[0].pwd
			c.mu.Unlock()

			u := fmt.Sprintf("http://%s/version", preferredHost)
			req, err := http.NewRequest("GET", u, nil)
			if err != nil {
				continue
			}
			req.Header.Set("Authorization", preferredPwd)
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			req = req.WithContext(ctx)
			resp, err := HTTPClient.Do(req)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					c.log("Preferred Lavalink node %s is back online. Reconnecting...", preferredHost)
					c.mu.Lock()
					if c.ws != nil {
						_ = c.ws.Close()
					}
					c.mu.Unlock()
				}
			}
			cancel()
		}
	}
}
