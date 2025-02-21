package main

import (
	"github.com/urfave/cli/v2"
)

var reportmarketfee = cli.Command{
	Name:  "reportmarketfee",
	Usage: "get a report of the fees collected for the trades of a market.",
	Flags: []cli.Flag{
		&cli.Uint64Flag{
			Name:  "page",
			Usage: "the number of the page to be listed. If omitted, the entire list is returned",
		},
		&cli.Uint64Flag{
			Name:  "page_size",
			Usage: "the size of the page",
			Value: 10,
		},
	},
	Action: reportMarketFeeAction,
}

func reportMarketFeeAction(ctx *cli.Context) error {
	printDeprecatedWarn("tdex market reportfee")
	return nil
}
