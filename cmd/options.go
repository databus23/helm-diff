package cmd

import (
	"github.com/databus23/helm-diff/v3/diff"
	"github.com/spf13/pflag"
)

func AddDiffOptions(f *pflag.FlagSet, o *diff.Options) {
	f.BoolP("suppress-secrets", "q", false, "suppress secrets in the output")
	f.BoolVar(&o.ShowSecrets, "show-secrets", false, "do not redact secret values in the output")
	f.StringArrayVar(&o.SuppressedKinds, "suppress", []string{}, "allows suppression of the values listed in the diff output")
	f.IntVarP(&o.OutputContext, "context", "C", -1, "output NUM lines of context around changes")
	f.StringVar(&o.OutputFormat, "output", "diff", "Possible values: diff, simple, template. When set to \"template\", use the env var HELM_DIFF_TPL to specify the template.")
	f.BoolVar(&o.StripTrailingCR, "strip-trailing-cr", false, "strip trailing carriage return on input")
}

func ProcessDiffOptions(f *pflag.FlagSet, o *diff.Options) {
	if q, _ := f.GetBool("suppress-secrets"); q {
		o.SuppressedKinds = append(o.SuppressedKinds, "Secret")
	}
}
