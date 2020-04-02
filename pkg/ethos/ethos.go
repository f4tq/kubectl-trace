package ethos

import (
	"fmt"
//	"github.com/iovisor/kubectl-trace/pkg/cmd"

	"github.com/iovisor/kubectl-trace/pkg/factory"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

var (
	ethosLong = `Configure, execute, and manage bpftrace programs.

These commands help you trace existing application resources.
	`
	ethosExamples = `
  All scripts should begin with #!/usr/bin/env python|python3|bpftrace etc

  # Execute a bpftrace program from file on a specific node
  %[1]s ethos run mynode.aws.internal bitesize.py
  
  %[1]s ethos run kubernetes-node-emt8.c.myproject.internal read.bt

  # Get all bpftrace programs in all namespaces
  %[1]s ethos get --all-namespaces

  # Delete all bpftrace programs in a specific namespace
  %[1]s ethos delete -n my-namespace
`
)

// TraceOptions ...
type EthosOptions struct {
	configFlags *genericclioptions.ConfigFlags

	genericclioptions.IOStreams
}

// NewTraceOptions provides an instance of TraceOptions with default values.
func NewEthosOptions(streams genericclioptions.IOStreams) *EthosOptions {
	return &EthosOptions{
		configFlags: genericclioptions.NewConfigFlags(false),

		IOStreams: streams,
	}
}

// NewTraceCommand creates the trace command and its nested children.
func NewEthosCommand(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewEthosOptions(streams)

	cmd := &cobra.Command{
		Use:                   "ethos",
		DisableFlagsInUseLine: true,
		Short:                 `Execute and manage ethos monitoring programs`, // Wrap with i18n.T()
		Long:                  ethosLong,                              // Wrap with templates.LongDesc()
		Example:               fmt.Sprintf(ethosExamples, "kubectl"),  // Wrap with templates.Examples()
		PersistentPreRun: func(c *cobra.Command, args []string) {
			c.SetOutput(streams.ErrOut)
		},
		Run: func(c *cobra.Command, args []string) {
			cobra.NoArgs(c, args)
			c.Help()
		},
	}

	flags := cmd.PersistentFlags()
	o.configFlags.AddFlags(flags)

	matchVersionFlags := factory.NewMatchVersionFlags(o.configFlags)
	matchVersionFlags.AddFlags(flags)

	// flags.AddGoFlagSet(flag.CommandLine) // todo(leodido) > evaluate whether we need this or not

	f := factory.NewFactory(matchVersionFlags)

	cmd.AddCommand(NewRunCommand(f, streams))
	/*
	cmd.AddCommand(NewGetCommand(f, streams))
	cmd.AddCommand(NewAttachCommand(f, streams))
	cmd.AddCommand(NewDeleteCommand(f, streams))
	cmd.AddCommand(NewVersionCommand(streams))
	cmd.AddCommand(NewLogCommand(f, streams))
*/
	// Override help on all the commands tree
	walk(cmd, func(c *cobra.Command) {
		c.Flags().BoolP("help", "h", false, fmt.Sprintf("Help for the %s command", c.Name()))
	})

	return cmd
}

// walk calls f for c and all of its children.
func walk(c *cobra.Command, f func(*cobra.Command)) {
	f(c)
	for _, c := range c.Commands() {
		walk(c, f)
	}
}
