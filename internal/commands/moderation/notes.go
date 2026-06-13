package moderation

import (
	"fmt"
	"skyvern/internal/manager"
	"skyvern/internal/moderation"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func init() {
	manager.RegisterHelp("notes", []manager.HelpPage{
		{
			Command:     "Notes View",
			Syntax:      ".notes <member>",
			Description: "View notes on a member.",
		},
		{
			Command:     "Notes Add",
			Syntax:      ".notes add <member> <note>",
			Description: "Add a note for a member.",
		},
		{
			Command:     "Notes Remove",
			Syntax:      ".notes remove <member> <id>",
			Description: "Removes a note for a member.",
		},
		{
			Command:     "Notes Clear",
			Syntax:      ".notes clear <member>",
			Description: "Clears all notes for a member.",
		},
	})
}

var Notes = &manager.Command{
	Trigger:     "notes",
	Aliases:     []string{"note"},
	Name:        "notes",
	Description: "View and manage moderator notes on server members",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageMessages) {
			return ctx.Reply("[!] You need Manage Messages permission.")
		}
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("notes")
		}

		gid := ctx.GuildID()
		sub := strings.ToLower(ctx.Args[0])

		if sub == "add" {
			if len(ctx.Args) < 3 {
				return ctx.Reply("Usage: `.notes add <member> <note>`")
			}
			m, err := moderation.ResolveMember(ctx.Session, gid, ctx.Args[1])
			if err != nil || m == nil {
				return ctx.Reply("[!] Could not resolve member.")
			}
			noteText := strings.Join(ctx.Args[2:], " ")
			if noteText == "" {
				return ctx.Reply("[!] Note content cannot be empty.")
			}
			id, err := ctx.DB.SaveNote(gid, m.User.ID, noteText, ctx.AuthorID())
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to save note: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Saved note (ID: `%s`) for **%s**.", id, m.User.Username))
		}

		if sub == "remove" || sub == "delete" {
			if len(ctx.Args) < 3 {
				return ctx.Reply("Usage: `.notes remove <member> <id>`")
			}
			m, err := moderation.ResolveMember(ctx.Session, gid, ctx.Args[1])
			if err != nil || m == nil {
				return ctx.Reply("[!] Could not resolve member.")
			}
			noteID := ctx.Args[2]
			err = ctx.DB.DeleteNote(gid, m.User.ID, noteID)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to delete note: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Deleted note `%s` for **%s**.", noteID, m.User.Username))
		}

		if sub == "clear" {
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.notes clear <member>`")
			}
			m, err := moderation.ResolveMember(ctx.Session, gid, ctx.Args[1])
			if err != nil || m == nil {
				return ctx.Reply("[!] Could not resolve member.")
			}
			err = ctx.DB.ClearNotes(gid, m.User.ID)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to clear notes: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Cleared all notes for **%s**.", m.User.Username))
		}

		// Otherwise, view notes list
		m, err := moderation.ResolveMember(ctx.Session, gid, ctx.Args[0])
		if err != nil || m == nil {
			return ctx.Reply("[!] Could not resolve member.")
		}
		list, err := ctx.DB.ListNotes(gid, m.User.ID)
		if err != nil || len(list) == 0 {
			return ctx.Reply(fmt.Sprintf("[*] No notes recorded for **%s**.", m.User.Username))
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Notes for **%s** (%d notes):\n\n", m.User.Username, len(list)))
		for _, note := range list {
			modName := note.Moderator
			if modUser, err := ctx.Session.User(note.Moderator); err == nil && modUser != nil {
				modName = modUser.Username
			}
			dateStr := note.Timestamp.Format("2006-01-02 15:04")
			sb.WriteString(fmt.Sprintf("- `ID: %s` | **%s** | Mod: **%s**\n  > %s\n", note.ID, dateStr, modName, note.Text))
		}
		return ctx.Reply(sb.String())
	},
}
