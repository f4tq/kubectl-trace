package meta

const (
	// TracePrefix is the prefix to identify objects created by this tool
	TracePrefix = "kubectl-trace-"
	// TraceIDLabelKey is a meta to annotate objects created by this tool
	TraceIDLabelKey = "iovisor.org/kubectl-trace-id"
	// TraceLabelKey is a meta to annotate objects created by this tool
	TraceLabelKey = "iovisor.org/kubectl-trace"

	// ObjectNamePrefix is the prefix used for objects created by kubectl-trace
	ObjectNamePrefix = "kubectl-trace-"

/*
	EthosPrefix = "kubectl-ethos-"
	// TraceIDLabelKey is a meta to annotate objects created by this tool
	EthosIDLabelKey = "adobe.com/kubectl-ethos-id"
	// TraceLabelKey is a meta to annotate objects created by this tool
	EthosLabelKey = "adobe.com/kubectl-ethos"

	// ObjectNamePrefix is the prefix used for objects created by kubectl-trace
	EthosObjectNamePrefix = "kubectl-ethos-"
*/
	EthosPrefix = TracePrefix
	// TraceIDLabelKey is a meta to annotate objects created by this tool
	EthosIDLabelKey = TraceIDLabelKey
	// TraceLabelKey is a meta to annotate objects created by this tool
	EthosLabelKey = TraceLabelKey

	// ObjectNamePrefix is the prefix used for objects created by kubectl-trace
	EthosObjectNamePrefix = ObjectNamePrefix

)
