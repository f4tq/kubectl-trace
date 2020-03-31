package main

import (
	"os"

	"github.com/iovisor/kubectl-trace/pkg/ethos"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	_ "k8s.io/client-go/plugin/pkg/client/auth/azure"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

func main() {
	flags := pflag.NewFlagSet("kubectl-ethos", pflag.ExitOnError)
	pflag.CommandLine = flags

	streams := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}

	root := ethos.NewEthosCommand(streams)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}