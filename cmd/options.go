package cmd

import (
	"github.com/spf13/pflag"

	"github.com/databus23/helm-diff/v3/diff"
)

// AddDiffOptions adds flags for the various consolidated options to the functions in the diff package
func AddDiffOptions(f *pflag.FlagSet, o *diff.Options) {
	f.BoolP("suppress-secrets", "q", false, "suppress secrets in the output")
	f.BoolVar(&o.ShowSecrets, "show-secrets", false, "do not redact secret values in the output")
	f.StringArrayVar(&o.SuppressedKinds, "suppress", []string{}, "allows suppression of the values listed in the diff output")
	f.IntVarP(&o.OutputContext, "context", "C", -1, "output NUM lines of context around changes")
	f.StringVar(&o.OutputFormat, "output", "diff", "Possible values: diff, simple, template, dyff. When set to \"template\", use the env var HELM_DIFF_TPL to specify the template.")
	f.BoolVar(&o.StripTrailingCR, "strip-trailing-cr", false, "strip trailing carriage return on input")
	f.Float32VarP(&o.FindRenames, "find-renames", "D", 0, "Enable rename detection if set to any value greater than 0. If specified, the value denotes the maximum fraction of changed content as lines added + removed compared to total lines in a diff for considering it a rename. Only objects of the same Kind are attempted to be matched")
}

// ProcessDiffOptions processes the set flags and handles possible interactions between them
func ProcessDiffOptions(f *pflag.FlagSet, o *diff.Options) {
	if q, _ := f.GetBool("suppress-secrets"); q {
		o.SuppressedKinds = append(o.SuppressedKinds, "Secret")
	}
}
