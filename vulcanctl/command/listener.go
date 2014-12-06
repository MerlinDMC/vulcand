package command

import (
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/mailgun/vulcand/engine"
)

func NewListenerCommand(cmd *Command) cli.Command {
	return cli.Command{
		Name:  "listener",
		Usage: "Operations with socket listeners",
		Subcommands: []cli.Command{
			{
				Name:  "upsert",
				Usage: "Update or insert a listener",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "id", Usage: "id"},
					cli.StringFlag{Name: "proto", Usage: "protocol, either http or https"},
					cli.StringFlag{Name: "net", Value: "tcp", Usage: "network, tcp or unix"},
					cli.StringFlag{Name: "addr", Value: "tcp", Usage: "address to bind to, e.g. 'localhost:31000'"},
				},
				Action: cmd.upsertListenerAction,
			},
			{
				Name:   "rm",
				Usage:  "Remove a listener",
				Action: cmd.deleteListenerAction,
				Flags: []cli.Flag{
					cli.StringFlag{Name: "id", Usage: "id"},
				},
			},
		},
	}
}

func (cmd *Command) upsertListenerAction(c *cli.Context) {
	listener, err := engine.NewListener(c.String("id"), c.String("proto"), c.String("net"), c.String("addr"))
	if err != nil {
		cmd.printError(err)
	}
	if err := cmd.client.UpsertListener(*listener); err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOk("listener upserted")
}

func (cmd *Command) deleteListenerAction(c *cli.Context) {
	if err := cmd.client.DeleteListener(engine.ListenerKey{Id: c.String("id")}); err != nil {
		cmd.printError(err)
	}
	cmd.printOk("listener deleted")
}
