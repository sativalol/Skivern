package utility

import (
	"encoding/json"
	"fmt"
	"net/http"
	"skyvern/internal/manager"
	"strconv"
	"strings"
)

var Convert = &manager.Command{
	Trigger:     "convert",
	Aliases:     []string{"currency"},
	Name:        "convert",
	Description: "Convert currency value to another currency",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) < 3 {
			return ctx.Reply("Usage: `.convert <amount> <from_currency> <to_currency>` (e.g. `.convert 100 USD EUR`)")
		}

		amountStr := ctx.Args[0]
		amount, err := strconv.ParseFloat(amountStr, 64)
		if err != nil {
			return ctx.Reply("[!] Invalid amount. Must be a valid number.")
		}

		from := strings.ToUpper(ctx.Args[1])
		to := strings.ToUpper(ctx.Args[2])

		apiURL := fmt.Sprintf("https://api.exchangerate-api.com/v4/latest/%s", from)
		resp, err := http.Get(apiURL)
		if err != nil {
			return ctx.Reply("[!] Failed to fetch exchange rates. Try again later.")
		}
		defer resp.Body.Close()

		var res struct {
			Rates map[string]float64 `json:"rates"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			return ctx.Reply("[!] Invalid response from exchange rate service.")
		}

		rate, exists := res.Rates[to]
		if !exists {
			return ctx.Reply(fmt.Sprintf("[!] Target currency `%s` is not supported.", to))
		}

		converted := amount * rate
		return ctx.Reply(fmt.Sprintf("[*] **%.2f %s** = **%.2f %s** (Rate: %.4f)", amount, from, converted, to, rate))
	},
}
