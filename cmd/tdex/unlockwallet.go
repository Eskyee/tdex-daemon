package main

import (
	"context"
	"fmt"

	daemonv1 "github.com/tdex-network/tdex-daemon/api-spec/protobuf/gen/tdex-daemon/v1"

	"github.com/urfave/cli/v2"
)

var unlockwallet = cli.Command{
	Name:  "unlock",
	Usage: "unlock the daemon wallet with the given password",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "password",
			Usage: "the password used to encrypt the mnemonic",
			Value: "",
		},
	},
	Action: unlockWalletAction,
}

func unlockWalletAction(ctx *cli.Context) error {
	client, cleanup, err := getUnlockerClient(ctx)
	if err != nil {
		return err
	}
	defer cleanup()

	_, err = client.UnlockWallet(
		context.Background(), &daemonv1.UnlockWalletRequest{
			WalletPassword: []byte(ctx.String("password")),
		},
	)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Wallet is unlocked")
	return nil
}
