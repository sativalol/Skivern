package moderation

import (
	"fmt"
	"skyvern/internal/manager"
	"skyvern/internal/moderation"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func init() {
	manager.RegisterHelp("thread", []manager.HelpPage{
		{
			Command:     "Thread Lock",
			Syntax:      ".thread lock [thread] [reason]",
			Description: "Lock a thread or forum post.",
		},
		{
			Command:     "Thread Unlock",
			Syntax:      ".thread unlock [thread] [reason]",
			Description: "Unlock a thread or forum post.",
		},
		{
			Command:     "Thread Rename",
			Syntax:      ".thread rename [thread] <new_name>",
			Description: "Rename a thread or forum post.",
		},
		{
			Command:     "Thread Add",
			Syntax:      ".thread add <thread> <member>",
			Description: "Add a member to a thread.",
		},
		{
			Command:     "Thread Remove",
			Syntax:      ".thread remove <thread> <member>",
			Description: "Remove a member from a thread.",
		},
		{
			Command:     "Thread Watch",
			Syntax:      ".thread watch [thread]",
			Description: "Toggle watching a thread to keep it active.",
		},
		{
			Command:     "Thread Watch List",
			Syntax:      ".thread watch list",
			Description: "View list of watched threads.",
		},
	})
}

var Thread = &manager.Command{
	Trigger:     "thread",
	Name:        "thread",
	Description: "Manage thread channels and forum posts",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("thread")
		}

		gid := ctx.GuildID()
		sub := strings.ToLower(ctx.Args[0])

		switch sub {
		case "lock", "unlock", "rename", "add", "remove", "watch":
			// We will check permissions in sub-commands
		default:
			return ctx.SendHelp("thread")
		}

		switch sub {
		case "watch":
			if !checkPerm(ctx, discordgo.PermissionManageChannels) {
				return ctx.Reply("[!] You need Manage Channels permission.")
			}
			if len(ctx.Args) > 1 && strings.ToLower(ctx.Args[1]) == "list" {
				list, _ := ctx.DB.ListWatchedThreads(gid)
				if len(list) == 0 {
					return ctx.Reply("[*] No threads are currently watched in this server.")
				}
				var sb strings.Builder
				sb.WriteString("Watched Threads:\n\n")
				for _, tid := range list {
					sb.WriteString(fmt.Sprintf("- <#%s> (`%s`)\n", tid, tid))
				}
				return ctx.Reply(sb.String())
			}

			target := ctx.ChanID()
			if len(ctx.Args) > 1 {
				if ch, err := moderation.ResolveChannel(ctx.Session, gid, ctx.Args[1]); err == nil && ch != nil {
					target = ch.ID
				} else {
					target = strings.Trim(ctx.Args[1], "<#>")
				}
			}

			ch, err := ctx.Session.Channel(target)
			if err != nil || ch.GuildID != gid {
				return ctx.Reply("[!] Invalid thread channel.")
			}

			watched, _ := ctx.DB.IsWatchedThread(gid, target)
			if watched {
				_ = ctx.DB.DeleteWatchedThread(gid, target)
				return ctx.Reply(fmt.Sprintf("[+] Stopped watching thread <#%s>.", target))
			} else {
				_ = ctx.DB.SaveWatchedThread(gid, target)
				return ctx.Reply(fmt.Sprintf("[+] Now watching thread <#%s>. If it gets archived, it will be automatically unarchived.", target))
			}

		case "lock":
			if !checkPerm(ctx, discordgo.PermissionManageThreads) {
				return ctx.Reply("[!] You need Manage Threads permission.")
			}
			target := ctx.ChanID()
			reason := "Thread lock"
			argIdx := 1

			if len(ctx.Args) > 1 {
				if ch, err := moderation.ResolveChannel(ctx.Session, gid, ctx.Args[1]); err == nil && ch != nil {
					target = ch.ID
					argIdx = 2
				} else if strings.HasPrefix(ctx.Args[1], "<#") || len(ctx.Args[1]) >= 17 {
					target = strings.Trim(ctx.Args[1], "<#>")
					argIdx = 2
				}
			}

			if len(ctx.Args) > argIdx {
				reason = strings.Join(ctx.Args[argIdx:], " ")
			}

			locked := true
			_, err := ctx.Session.ChannelEditComplex(target, &discordgo.ChannelEdit{
				Locked: &locked,
			})
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to lock thread: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Locked thread <#%s>. Reason: %s", target, reason))

		case "unlock":
			if !checkPerm(ctx, discordgo.PermissionManageThreads) {
				return ctx.Reply("[!] You need Manage Threads permission.")
			}
			target := ctx.ChanID()
			reason := "Thread unlock"
			argIdx := 1

			if len(ctx.Args) > 1 {
				if ch, err := moderation.ResolveChannel(ctx.Session, gid, ctx.Args[1]); err == nil && ch != nil {
					target = ch.ID
					argIdx = 2
				} else if strings.HasPrefix(ctx.Args[1], "<#") || len(ctx.Args[1]) >= 17 {
					target = strings.Trim(ctx.Args[1], "<#>")
					argIdx = 2
				}
			}

			if len(ctx.Args) > argIdx {
				reason = strings.Join(ctx.Args[argIdx:], " ")
			}

			locked := false
			_, err := ctx.Session.ChannelEditComplex(target, &discordgo.ChannelEdit{
				Locked: &locked,
			})
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to unlock thread: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Unlocked thread <#%s>. Reason: %s", target, reason))

		case "rename":
			if !checkPerm(ctx, discordgo.PermissionManageThreads) {
				return ctx.Reply("[!] You need Manage Threads permission.")
			}
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.thread rename [thread] <new_name>`")
			}

			target := ctx.ChanID()
			name := strings.Join(ctx.Args[1:], " ")

			if len(ctx.Args) > 2 {
				if ch, err := moderation.ResolveChannel(ctx.Session, gid, ctx.Args[1]); err == nil && ch != nil {
					target = ch.ID
					name = strings.Join(ctx.Args[2:], " ")
				} else if strings.HasPrefix(ctx.Args[1], "<#") || len(ctx.Args[1]) >= 17 {
					target = strings.Trim(ctx.Args[1], "<#>")
					name = strings.Join(ctx.Args[2:], " ")
				}
			}

			if strings.TrimSpace(name) == "" {
				return ctx.Reply("[!] Name cannot be empty.")
			}

			_, err := ctx.Session.ChannelEdit(target, &discordgo.ChannelEdit{
				Name: strings.TrimSpace(name),
			})
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to rename thread: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Renamed thread to `%s`.", name))

		case "add":
			if !checkPerm(ctx, discordgo.PermissionManageThreads) {
				return ctx.Reply("[!] You need Manage Threads permission.")
			}
			if len(ctx.Args) < 3 {
				return ctx.Reply("Usage: `.thread add <thread> <member>`")
			}

			chID := strings.Trim(ctx.Args[1], "<#>")
			memID := strings.Trim(ctx.Args[2], "<@!?")

			err := ctx.Session.ThreadMemberAdd(chID, memID)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to add member to thread: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Added <@%s> to thread <#%s>.", memID, chID))

		case "remove":
			if !checkPerm(ctx, discordgo.PermissionManageThreads) {
				return ctx.Reply("[!] You need Manage Threads permission.")
			}
			if len(ctx.Args) < 3 {
				return ctx.Reply("Usage: `.thread remove <thread> <member>`")
			}

			chID := strings.Trim(ctx.Args[1], "<#>")
			memID := strings.Trim(ctx.Args[2], "<@!?")

			err := ctx.Session.ThreadMemberRemove(chID, memID)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to remove member from thread: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Removed <@%s> from thread <#%s>.", memID, chID))
		}

		return nil
	},
}
