package utility

import (
	"fmt"
	"net"
	"skyvern/internal/manager"
	"strings"
)

var Dig = &manager.Command{
	Trigger:     "dig",
	Name:        "dig",
	Description: "Perform a DNS lookup for a domain",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage: `.dig <domain>`")
		}

		domain := ctx.Args[0]
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("**DNS Lookup for %s**\n\n", domain))

		// A / AAAA
		ips, err := net.LookupIP(domain)
		if err == nil && len(ips) > 0 {
			sb.WriteString("__IP Records (A/AAAA)__\n")
			for _, ip := range ips {
				sb.WriteString(fmt.Sprintf("- %s\n", ip.String()))
			}
			sb.WriteString("\n")
		}

		// MX
		mxs, err := net.LookupMX(domain)
		if err == nil && len(mxs) > 0 {
			sb.WriteString("__Mail Exchanger (MX)__\n")
			for _, mx := range mxs {
				sb.WriteString(fmt.Sprintf("- %s (Pref: %d)\n", mx.Host, mx.Pref))
			}
			sb.WriteString("\n")
		}

		// TXT
		txts, err := net.LookupTXT(domain)
		if err == nil && len(txts) > 0 {
			sb.WriteString("__TXT Records__\n")
			for _, txt := range txts {
				sb.WriteString(fmt.Sprintf("- `%s`\n", txt))
			}
			sb.WriteString("\n")
		}

		// CNAME
		cname, err := net.LookupCNAME(domain)
		if err == nil && cname != "" && !strings.HasPrefix(cname, domain) {
			sb.WriteString(fmt.Sprintf("__CNAME Record__\n- %s\n", cname))
		}

		res := sb.String()
		if len(res) > 2000 {
			res = res[:1990] + "..."
		}
		if res == fmt.Sprintf("**DNS Lookup for %s**\n\n", domain) {
			return ctx.Reply("[!] No DNS records resolved or invalid domain.")
		}

		return ctx.Reply(res)
	},
}
