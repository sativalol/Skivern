package general

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"skyvern/internal/config"
	"skyvern/internal/manager"
	"sort"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// FirstMessage
var FirstMessage = &manager.Command{
	Trigger:     "firstmessage",
	Aliases:     []string{"fm", "oldest"},
	Name:        "firstmessage",
	Description: "Finds and links to the first message ever sent in this channel",
	Category:    "general",
	Execute: func(ctx *manager.CommandContext) error {
		msgs, err := ctx.Session.ChannelMessages(ctx.ChanID(), 1, "", ctx.ChanID(), "")
		if err != nil || len(msgs) == 0 {
			return ctx.Reply("[!] Could not find any messages in this channel.")
		}
		first := msgs[0]
		link := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", ctx.GuildID(), ctx.ChanID(), first.ID)
		emb := config.Build(ctx.Cfg, config.EmbedOpt{
			Title:       "First Message in Channel",
			Description: fmt.Sprintf("**Author:** <@%s>\n**Content:** %s\n\n[Jump to Message](%s)", first.Author.ID, first.Content, link),
		})
		return ctx.Respond(emb)
	},
}

// InRole Pagination component
func HandleInRoleComponent(s *discordgo.Session, i *discordgo.InteractionCreate, mgr *manager.Manager) {
	parts := strings.Split(i.MessageComponentData().CustomID, ":")
	if len(parts) < 3 {
		return
	}
	rid := parts[1]
	page, _ := strconv.Atoi(parts[2])

	gid := i.GuildID
	members, err := s.GuildMembers(gid, "", 1000)
	if err != nil {
		return
	}

	var inRoleUsers []string
	for _, m := range members {
		for _, r := range m.Roles {
			if r == rid {
				inRoleUsers = append(inRoleUsers, fmt.Sprintf("<@%s> (`%s`)", m.User.ID, m.User.ID))
				break
			}
		}
	}

	pages := (len(inRoleUsers) + 9) / 10
	if pages == 0 {
		pages = 1
	}
	if page < 1 {
		page = 1
	}
	if page > pages {
		page = pages
	}

	start := (page - 1) * 10
	end := start + 10
	if end > len(inRoleUsers) {
		end = len(inRoleUsers)
	}

	slice := inRoleUsers[start:end]

	resCfg, _ := mgr.ResolvedCfgFor(s.State.User.ID)

	emb := config.Build(resCfg, config.EmbedOpt{
		Title:       "Users in Role",
		Description: fmt.Sprintf("Showing page **%d** of **%d**:\n\n%s", page, pages, strings.Join(slice, "\n")),
	})

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{emb},
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "◀",
							Style:    discordgo.SecondaryButton,
							CustomID: fmt.Sprintf("inrole:%s:%d", rid, page-1),
							Disabled: page <= 1,
						},
						discordgo.Button{
							Label:    "▶",
							Style:    discordgo.SecondaryButton,
							CustomID: fmt.Sprintf("inrole:%s:%d", rid, page+1),
							Disabled: page >= pages,
						},
					},
				},
			},
		},
	})
}

var InRole = &manager.Command{
	Trigger:     "inrole",
	Name:        "inrole",
	Description: "List users who have a specific role with pagination",
	Category:    "general",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage: .inrole <role>")
		}

		gid := ctx.GuildID()
		roleArg := strings.Join(ctx.Args, " ")
		rid := resolveRole(ctx.Session, gid, roleArg)
		if rid == "" {
			return ctx.Reply("[!] Could not resolve role.")
		}

		members, err := ctx.Session.GuildMembers(gid, "", 1000)
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to fetch members: %v", err))
		}

		var inRoleUsers []string
		for _, m := range members {
			for _, r := range m.Roles {
				if r == rid {
					inRoleUsers = append(inRoleUsers, fmt.Sprintf("<@%s> (`%s`)", m.User.ID, m.User.ID))
					break
				}
			}
		}

		if len(inRoleUsers) == 0 {
			return ctx.Reply("[*] No users currently have this role.")
		}

		pages := (len(inRoleUsers) + 9) / 10
		end := 10
		if end > len(inRoleUsers) {
			end = len(inRoleUsers)
		}

		emb := config.Build(ctx.Cfg, config.EmbedOpt{
			Title:       "Users in Role",
			Description: fmt.Sprintf("Showing page **1** of **%d**:\n\n%s", pages, strings.Join(inRoleUsers[0:end], "\n")),
		})

		return ctx.Respond(emb)
	},
}

// Math Engine
type parser struct {
	expr string
	pos  int
}

func (p *parser) peek() byte {
	if p.pos >= len(p.expr) {
		return 0
	}
	return p.expr[p.pos]
}

func (p *parser) next() byte {
	b := p.peek()
	if b != 0 {
		p.pos++
	}
	return b
}

func (p *parser) consumeSpace() {
	for p.peek() == ' ' {
		p.pos++
	}
}

func (p *parser) parseExpression() (float64, error) {
	val, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for {
		p.consumeSpace()
		op := p.peek()
		if op == '+' || op == '-' {
			p.next()
			rhs, err := p.parseTerm()
			if err != nil {
				return 0, err
			}
			if op == '+' {
				val += rhs
			} else {
				val -= rhs
			}
		} else {
			break
		}
	}
	return val, nil
}

func (p *parser) parseTerm() (float64, error) {
	val, err := p.parsePower()
	if err != nil {
		return 0, err
	}
	for {
		p.consumeSpace()
		op := p.peek()
		if op == '*' || op == '/' {
			p.next()
			rhs, err := p.parsePower()
			if err != nil {
				return 0, err
			}
			if op == '*' {
				val *= rhs
			} else {
				if rhs == 0 {
					return 0, fmt.Errorf("division by zero")
				}
				val /= rhs
			}
		} else {
			break
		}
	}
	return val, nil
}

func (p *parser) parsePower() (float64, error) {
	val, err := p.parseFactor()
	if err != nil {
		return 0, err
	}
	p.consumeSpace()
	if p.peek() == '^' {
		p.next()
		rhs, err := p.parsePower()
		if err != nil {
			return 0, err
		}
		val = math.Pow(val, rhs)
	}
	return val, nil
}

func (p *parser) parseFactor() (float64, error) {
	p.consumeSpace()
	b := p.peek()
	if b == '-' {
		p.next()
		val, err := p.parseFactor()
		return -val, err
	}
	if b == '+' {
		p.next()
		return p.parseFactor()
	}
	if b == '(' {
		p.next()
		val, err := p.parseExpression()
		if err != nil {
			return 0, err
		}
		p.consumeSpace()
		if p.next() != ')' {
			return 0, fmt.Errorf("missing closing parenthesis")
		}
		return val, nil
	}

	start := p.pos
	for {
		c := p.peek()
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			p.next()
		} else {
			break
		}
	}
	name := p.expr[start:p.pos]
	if name != "" {
		p.consumeSpace()
		if p.peek() != '(' {
			return 0, fmt.Errorf("expected function arguments")
		}
		p.next()
		arg, err := p.parseExpression()
		if err != nil {
			return 0, err
		}
		if p.next() != ')' {
			return 0, fmt.Errorf("missing closing parenthesis after function")
		}
		switch strings.ToLower(name) {
		case "sqrt":
			return math.Sqrt(arg), nil
		case "sin":
			return math.Sin(arg), nil
		case "cos":
			return math.Cos(arg), nil
		case "tan":
			return math.Tan(arg), nil
		case "log":
			return math.Log10(arg), nil
		case "ln":
			return math.Log(arg), nil
		case "abs":
			return math.Abs(arg), nil
		default:
			return 0, fmt.Errorf("unknown function %q", name)
		}
	}

	startNum := p.pos
	hasDot := false
	for {
		c := p.peek()
		if c >= '0' && c <= '9' {
			p.next()
		} else if c == '.' {
			if hasDot {
				break
			}
			hasDot = true
			p.next()
		} else {
			break
		}
	}
	if startNum == p.pos {
		return 0, fmt.Errorf("expected number")
	}
	num, err := strconv.ParseFloat(p.expr[startNum:p.pos], 64)
	if err != nil {
		return 0, err
	}
	return num, nil
}

var Math = &manager.Command{
	Trigger:     "math",
	Aliases:     []string{"calc"},
	Name:        "math",
	Description: "Professional Math Engine: K-12 to Advanced Engineering",
	Category:    "general",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage: .math <expression> (e.g. .math 2 * (10 / 5) ^ 3)")
		}

		expr := strings.Join(ctx.Args, "")
		p := &parser{expr: expr}
		res, err := p.parseExpression()
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Calculation Error: %v", err))
		}

		return ctx.Reply(fmt.Sprintf("[*] **Expression:** `%s` \n[+] **Result:** `%g` / `%f`", expr, res, res))
	},
}

// Messages
var Messages = &manager.Command{
	Trigger:     "messages",
	Aliases:     []string{"msgs", "msg"},
	Name:        "messages",
	Description: "Check message statistics and view message leaderboards",
	Category:    "general",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) > 0 {
			sub := strings.ToLower(ctx.Args[0])
			if sub == "leaderboard" || sub == "lb" {
				entries, err := ctx.DB.GetMessageLeaderboard(ctx.GuildID())
				if err != nil || len(entries) == 0 {
					return ctx.Reply("[*] Leaderboard is currently empty.")
				}
				sort.Slice(entries, func(i, j int) bool {
					return entries[i].Count > entries[j].Count
				})
				if len(entries) > 10 {
					entries = entries[:10]
				}
				var sb strings.Builder
				sb.WriteString("Message Count Leaderboard:\n\n")
				for idx, ent := range entries {
					sb.WriteString(fmt.Sprintf("%d. <@%s>: **%d** messages\n", idx+1, ent.UserID, ent.Count))
				}
				return ctx.Reply(sb.String())
			}
		}

		target := ctx.AuthorID()
		if len(ctx.Args) > 0 {
			usr, err := resolveUser(ctx.Session, ctx.GuildID(), ctx.Args[0])
			if err == nil && usr != nil {
				target = usr.ID
			}
		}
		count := ctx.DB.GetUserMessages(ctx.GuildID(), target)
		return ctx.Reply(fmt.Sprintf("<@%s> has sent **%d** messages in this server.", target, count))
	},
}

// Whoisweb
func isIP(query string) bool {
	for _, c := range query {
		if (c >= '0' && c <= '9') || c == '.' || c == ':' {
			continue
		}
		return false
	}
	return true
}

var WhoisWeb = &manager.Command{
	Trigger:     "whoisweb",
	Name:        "whoisweb",
	Description: "Perform a WHOIS lookup for an IP address or web domain",
	Category:    "general",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage: .whoisweb <domain/ip>")
		}

		query := ctx.Args[0]
		url := "https://rdap.org/domain/" + query
		if isIP(query) {
			url = "https://rdap.org/ip/" + query
		}

		resp, err := http.Get(url) // #nosec G107
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] WHOIS lookup failed: %v", err))
		}
		defer resp.Body.Close()

		if resp.StatusCode == 404 {
			return ctx.Reply("[!] Domain or IP address not found in RDAP registry.")
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return ctx.Reply("[!] Failed to read WHOIS response.")
		}

		var data map[string]any
		_ = json.Unmarshal(body, &data)

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("WHOIS Web Data for **%s**:\n\n", query))

		if ldh, ok := data["ldhName"].(string); ok {
			sb.WriteString(fmt.Sprintf("- **Name:** %s\n", ldh))
		}
		if status, ok := data["status"].([]any); ok {
			var stList []string
			for _, st := range status {
				if sStr, ok := st.(string); ok {
					stList = append(stList, sStr)
				}
			}
			sb.WriteString(fmt.Sprintf("- **Status:** %s\n", strings.Join(stList, ", ")))
		}
		if events, ok := data["events"].([]any); ok {
			for _, ev := range events {
				if evMap, ok := ev.(map[string]any); ok {
					act, _ := evMap["eventAction"].(string)
					date, _ := evMap["eventDate"].(string)
					if act != "" && date != "" {
						sb.WriteString(fmt.Sprintf("- **%s:** %s\n", act, date))
					}
				}
			}
		}

		return ctx.Reply(sb.String())
	},
}
