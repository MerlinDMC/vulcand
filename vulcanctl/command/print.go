package command

import (
	"bytes"
	"fmt"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/buger/goterm"
	"github.com/mailgun/vulcand/engine"
)

func (cmd *Command) printResult(format string, in interface{}, err error) {
	if err != nil {
		cmd.printError(err)
	} else {
		cmd.printOk(format, fmt.Sprintf("%v", in))
	}
}

func (cmd *Command) printStatus(in interface{}, err error) {
	if err != nil {
		cmd.printError(err)
	} else {
		cmd.printOk("%s", in)
	}
}

func (cmd *Command) printError(err error) {
	fmt.Fprint(cmd.out, goterm.Color(fmt.Sprintf("ERROR: %s", err), goterm.RED)+"\n")
}

func (cmd *Command) printOk(message string, params ...interface{}) {
	fmt.Fprintf(cmd.out, goterm.Color(fmt.Sprintf("OK: %s\n", fmt.Sprintf(message, params...)), goterm.GREEN)+"\n")
}

func (cmd *Command) printInfo(message string, params ...interface{}) {
	fmt.Fprintf(cmd.out, "INFO: %s\n", fmt.Sprintf(message, params...))
}

func (cmd *Command) printHosts(hosts []engine.Host) {
	fmt.Fprintf(cmd.out, "\n")
	printTree(cmd.out, hostsView(hosts), 0, true, "")
}

func (cmd *Command) printHost(host *engine.Host) {
	fmt.Fprintf(cmd.out, "\n")
	printTree(cmd.out, hostView(host), 0, true, "")
}

func (cmd *Command) printOverview(frontend []engine.Frontend, servers []engine.Server) {
	out := &bytes.Buffer{}
	fmt.Fprintf(out, "[Frontend]\n\n")
	fmt.Fprintf(out, frontendOverview(frontend))
	fmt.Fprintf(cmd.out, out.String())

	out = &bytes.Buffer{}
	fmt.Fprintf(out, "\n\n[Servers]\n\n")
	fmt.Fprintf(out, serversOverview(servers))
	fmt.Fprintf(cmd.out, out.String())
}

func (cmd *Command) printBackends(backends []engine.Backend) {
	fmt.Fprintf(cmd.out, "\n")
	printTree(cmd.out, backendsView(backends), 0, true, "")
}

func (cmd *Command) printBackend(backend *engine.Backend) {
	fmt.Fprintf(cmd.out, "\n")
	printTree(cmd.out, backendView(backend), 0, true, "")
}

func (cmd *Command) printFrontend(l *engine.Frontend) {
	fmt.Fprintf(cmd.out, "\n")
	printTree(cmd.out, frontendView(l), 0, true, "")
}
