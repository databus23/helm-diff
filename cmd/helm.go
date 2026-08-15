package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// Source: cmd/helm/install.go

type valueFiles []string

func (v *valueFiles) String() string {
	return fmt.Sprint(*v)
}

// Ensures all valuesFiles exist
func (v *valueFiles) Valid() error {
	errStr := ""
	for _, valuesFile := range *v {
		if strings.TrimSpace(valuesFile) != "-" {
			if _, err := os.Stat(valuesFile); os.IsNotExist(err) {
				errStr += err.Error()
			}
		}
	}

	if errStr == "" {
		return nil
	}

	return errors.New(errStr)
}

func (v *valueFiles) Type() string {
	return "valueFiles"
}

func (v *valueFiles) Set(value string) error {
	for _, filePath := range strings.Split(value, ",") {
		*v = append(*v, filePath)
	}
	return nil
}

// Source: cmd/helm/helm.go

func checkArgsLength(argsReceived int, requiredArgs ...string) error {
	expectedNum := len(requiredArgs)
	if argsReceived != expectedNum {
		arg := "arguments"
		if expectedNum == 1 {
			arg = "argument"
		}
		return fmt.Errorf("This command needs %v %s: %s", expectedNum, arg, strings.Join(requiredArgs, ", "))
	}
	return nil
}

var (
	helmVersionRE  = regexp.MustCompile(`Version:\s*"([^"]+)"`)
	helmV4Version  = semver.MustParse("v4.0.0")
	minHelmVersion = semver.MustParse("v3.1.0-rc.1")
	// See https://github.com/helm/helm/pull/9426.
	minHelmVersionWithDryRunLookupSupport = semver.MustParse("v3.13.0")
	// The --reset-then-reuse-values flag for `helm upgrade` was added in
	// https://github.com/helm/helm/pull/9653 and released as part of Helm v3.14.0.
	minHelmVersionWithResetThenReuseValues = semver.MustParse("v3.14.0")
)

func getHelmVersion() (*semver.Version, error) {
	cmd := exec.Command(os.Getenv("HELM_BIN"), "version")
	debugPrint("Executing %s", strings.Join(cmd.Args, " "))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("Failed to run `%s version`: %w", os.Getenv("HELM_BIN"), err)
	}
	versionOutput := string(output)

	matches := helmVersionRE.FindStringSubmatch(versionOutput)
	if matches == nil {
		return nil, fmt.Errorf("Failed to find version in output %#v", versionOutput)
	}
	helmVersion, err := semver.NewVersion(matches[1])
	if err != nil {
		return nil, fmt.Errorf("Failed to parse version %#v: %w", matches[1], err)
	}

	return helmVersion, nil
}

func isHelmVersionAtLeast(versionToCompareTo *semver.Version) (bool, error) {
	helmVersion, err := getHelmVersion()

	if err != nil {
		return false, err
	}
	if helmVersion.LessThan(versionToCompareTo) {
		return false, nil
	}
	return true, nil
}

func isHelmVersionGreaterThanEqual(versionToCompareTo *semver.Version) (bool, error) {
	helmVersion, err := getHelmVersion()

	if err != nil {
		return false, err
	}

	return helmVersion.GreaterThanEqual(versionToCompareTo), nil
}

func compatibleHelm3Version() error {
	isCompatible, err := isHelmVersionAtLeast(minHelmVersion)
	if err != nil {
		return err
	}
	if !isCompatible {
		return fmt.Errorf("helm diff upgrade requires at least helm version %s", minHelmVersion.String())
	}
	return nil
}

// helmGetArgs builds the arguments for a `helm get <what> <release>` invocation.
//
// A revision of 0 means no --revision flag is passed, so helm defaults to the
// newest revision of the release regardless of its status.
func helmGetArgs(what, release string, revision int, namespace, kubeContext string) []string {
	args := []string{"get", what, release}
	if revision > 0 {
		args = append(args, "--revision", strconv.Itoa(revision))
	}
	if namespace != "" {
		args = append(args, "--namespace", namespace)
	}
	if kubeContext != "" {
		args = append(args, "--kube-context", kubeContext)
	}
	return args
}

// getRelease returns the manifest of the given release revision.
// A revision of 0 means the newest revision.
func getRelease(release string, revision int, namespace, kubeContext string) ([]byte, error) {
	cmd := exec.Command(os.Getenv("HELM_BIN"), helmGetArgs("manifest", release, revision, namespace, kubeContext)...)
	return outputWithRichError(cmd)
}

// getHooks returns the hooks of the given release revision.
// A revision of 0 means the newest revision.
func getHooks(release string, revision int, namespace, kubeContext string) ([]byte, error) {
	cmd := exec.Command(os.Getenv("HELM_BIN"), helmGetArgs("hooks", release, revision, namespace, kubeContext)...)
	return outputWithRichError(cmd)
}

func getChart(release, namespace, kubeContext string) (string, error) {
	args := []string{"get", "all", release, "--template", "{{.Release.Chart.Name}}"}
	if namespace != "" {
		args = append(args, "--namespace", namespace)
	}
	if kubeContext != "" {
		args = append(args, "--kube-context", kubeContext)
	}
	cmd := exec.Command(os.Getenv("HELM_BIN"), args...)
	out, err := outputWithRichError(cmd)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (d *diffCmd) template(isUpgrade bool) ([]byte, error) {
	flags := []string{}
	if d.devel {
		flags = append(flags, "--devel")
	}
	if d.noHooks && !d.useUpgradeDryRun {
		flags = append(flags, "--no-hooks")
	}
	if d.includeCRDs {
		flags = append(flags, "--include-crds")
	}
	if d.chartVersion != "" {
		flags = append(flags, "--version", d.chartVersion)
	}
	if d.chartRepo != "" {
		flags = append(flags, "--repo", d.chartRepo)
	}
	if d.namespace != "" {
		flags = append(flags, "--namespace", d.namespace)
	}
	if d.kubeContext != "" {
		flags = append(flags, "--kube-context", d.kubeContext)
	}
	if d.postRenderer != "" {
		flags = append(flags, "--post-renderer", d.postRenderer)
	}
	for _, arg := range d.postRendererArgs {
		flags = append(flags, "--post-renderer-args", arg)
	}
	if d.insecureSkipTLSVerify {
		flags = append(flags, "--insecure-skip-tls-verify")
	}
	// Helm automatically enable --reuse-values when there's no --set, --set-string, --set-json, --set-values, --set-file present.
	// Let's simulate that in helm-diff.
	// See https://medium.com/@kcatstack/understand-helm-upgrade-flags-reset-values-reuse-values-6e58ac8f127e
	shouldDefaultReusingValues := isUpgrade && len(d.values) == 0 && len(d.stringValues) == 0 && len(d.stringLiteralValues) == 0 && len(d.jsonValues) == 0 && len(d.valueFiles) == 0 && len(d.fileValues) == 0
	if (d.reuseValues || d.resetThenReuseValues || shouldDefaultReusingValues) && !d.resetValues && d.clusterAccessAllowed() {
		tmpfile, err := os.CreateTemp("", "existing-values")
		if err != nil {
			return nil, err
		}
		defer func() {
			_ = os.Remove(tmpfile.Name())
		}()
		// In the presence of --reuse-values (or --reset-values), --reset-then-reuse-values is ignored.
		if d.resetThenReuseValues && !d.reuseValues {
			var supported bool
			supported, err = isHelmVersionAtLeast(minHelmVersionWithResetThenReuseValues)
			if err != nil {
				return nil, err
			}
			if !supported {
				return nil, fmt.Errorf("Using --reset-then-reuse-values requires at least helm version %s", minHelmVersionWithResetThenReuseValues.String())
			}
			err = d.writeExistingValues(tmpfile, false)
		} else {
			err = d.writeExistingValues(tmpfile, true)
		}
		if err != nil {
			return nil, err
		}
		flags = append(flags, "--values", tmpfile.Name())
	}
	for _, value := range d.values {
		flags = append(flags, "--set", value)
	}
	for _, stringValue := range d.stringValues {
		flags = append(flags, "--set-string", stringValue)
	}
	for _, stringLiteralValue := range d.stringLiteralValues {
		flags = append(flags, "--set-literal", stringLiteralValue)
	}
	for _, jsonValue := range d.jsonValues {
		flags = append(flags, "--set-json", jsonValue)
	}
	for _, valueFile := range d.valueFiles {
		if strings.TrimSpace(valueFile) == "-" {
			bytes, err := io.ReadAll(os.Stdin)
			if err != nil {
				return nil, err
			}

			tmpfile, err := os.CreateTemp("", "helm-diff-stdin-values")
			if err != nil {
				return nil, err
			}
			defer func() {
				_ = os.Remove(tmpfile.Name())
			}()

			if _, err := tmpfile.Write(bytes); err != nil {
				_ = tmpfile.Close()
				return nil, err
			}

			if err := tmpfile.Close(); err != nil {
				return nil, err
			}

			flags = append(flags, "--values", tmpfile.Name())
		} else {
			flags = append(flags, "--values", valueFile)
		}
	}
	for _, fileValue := range d.fileValues {
		flags = append(flags, "--set-file", fileValue)
	}

	if d.disableOpenAPIValidation {
		flags = append(flags, "--disable-openapi-validation")
	}

	if d.enableDNS {
		flags = append(flags, "--enable-dns")
	}

	if d.SkipSchemaValidation {
		flags = append(flags, "--skip-schema-validation")
	}

	if d.takeOwnership {
		flags = append(flags, "--take-ownership")
	}

	isHelmV4, _ := isHelmVersionGreaterThanEqual(helmV4Version)
	flags = append(flags, serverSideFlags(isHelmV4, d.useUpgradeDryRun, d.serverSide)...)

	var (
		subcmd string
		filter func([]byte) []byte
	)

	// `--dry-run=client` or `--dry-run=server`?
	//
	// Or what's the relationoship between helm-diff's --dry-run flag,
	// HELM_DIFF_UPGRADE_DRY_RUN env var and the helm upgrade --dry-run flag?
	//
	// Read on to find out.
	if d.useUpgradeDryRun {
		if d.isAllowUnreleased() {
			// Otherwise you get the following error when this is a diff for a new install
			//   Error: UPGRADE FAILED: "$RELEASE_NAME" has no deployed releases
			flags = append(flags, "--install")
		}

		// If the program reaches here,
		// we are sure that the user wants to use the `helm upgrade --dry-run` command
		// for generating the manifests to be diffed.
		//
		// So the question is only whether to use `--dry-run=client` or `--dry-run=server`.
		//
		// As HELM_DIFF_UPGRADE_DRY_RUN is there for producing more complete and correct diff results,
		// we use --dry-run=server if the version of helm supports it.
		// Otherwise, we use --dry-run=client, as that's the best we can do.
		if useDryRunService, err := isHelmVersionAtLeast(minHelmVersionWithDryRunLookupSupport); err == nil && useDryRunService {
			flags = append(flags, "--dry-run=server")
		} else {
			flags = append(flags, "--dry-run")
		}
		subcmd = "upgrade"
		filter = func(s []byte) []byte {
			return extractManifestFromHelmUpgradeDryRunOutput(s, d.noHooks)
		}
	} else {
		if !d.disableValidation && d.clusterAccessAllowed() {
			isHelmV4, err := isHelmVersionGreaterThanEqual(helmV4Version)
			if err == nil && isHelmV4 {
				// For Helm v4, we use --dry-run=server by default to get correct .Capabilities.APIVersions.
				// This is only applied if the user hasn't explicitly set --dry-run=client, --dry-run=true, or --dry-run=false.
				// Note: dryRunMode="true" behaves like "client" (no cluster access).
				// Note: dryRunMode="false" behaves like "none" (no dry-run flag at all).
				// See https://github.com/databus23/helm-diff/issues/894
				if !slices.Contains([]string{dryRunNoOptDefVal, envTrue, envFalse}, d.dryRunMode) {
					flags = append(flags, "--dry-run=server")
				}
			} else {
				flags = append(flags, "--validate")
			}
		}

		if isUpgrade {
			flags = append(flags, "--is-upgrade")
		}

		for _, a := range d.extraAPIs {
			flags = append(flags, "--api-versions", a)
		}

		if d.kubeVersion != "" {
			flags = append(flags, "--kube-version", d.kubeVersion)
		}

		// To keep the full compatibility with older helm-diff versions,
		// we pass --dry-run to `helm template` only if Helm is greater than v3.13.0.
		if useDryRunService, err := isHelmVersionAtLeast(minHelmVersionWithDryRunLookupSupport); err == nil && useDryRunService {
			isHelmV4, _ := isHelmVersionGreaterThanEqual(helmV4Version)

			// For Helm v4, --dry-run=server may already have been added above when
			// clusterAccessAllowed() is true and d.dryRunMode is not "client", "true", or "false".
			// In that case (Helm v4 and d.dryRunMode not "client"/"true"/"false"), we skip adding any
			// additional dry-run flag here. In all other cases (Helm v3 or d.dryRunMode is "client"/"true"),
			// we add the appropriate dry-run mode below.
			// Note: dryRunMode="false" means no dry-run flag at all.
			if d.dryRunMode == envFalse {
				// "false" means no dry-run, skip adding any dry-run flag
			} else if !(isHelmV4 && !slices.Contains([]string{dryRunNoOptDefVal, envTrue}, d.dryRunMode)) {
				if d.dryRunMode == dryRunServer {
					// This is for security reasons!
					//
					// We give helm-template the additional cluster access for the helm `lookup` function
					// only if the user has explicitly requested it by --dry-run=server,
					//
					// In other words, although helm-diff-upgrade implies limited cluster access by default,
					// helm-diff-upgrade without a --dry-run flag does NOT imply
					// full cluster-access via helm-template --dry-run=server!
					flags = append(flags, "--dry-run=server")
				} else {
					// Since helm-diff 3.9.0 and helm 3.13.0, we pass --dry-run=client to `helm template` by default.
					// This doesn't make any difference for helm-diff itself,
					// because helm-template w/o flags is equivalent to helm-template --dry-run=client.
					// See https://github.com/helm/helm/pull/9426#discussion_r1181397259
					flags = append(flags, "--dry-run=client")
				}
			}
		}

		subcmd = "template"

		filter = func(s []byte) []byte {
			return stripOCIPullProgress(s)
		}
	}

	args := []string{subcmd, d.release, d.chart}
	args = append(args, flags...)

	cmd := exec.Command(os.Getenv("HELM_BIN"), args...)
	out, err := outputWithRichError(cmd)
	return filter(out), err
}

func (d *diffCmd) writeExistingValues(f *os.File, all bool) error {
	args := []string{"get", "values", d.release, "--output", "yaml"}
	if all {
		args = append(args, "--all")
	}
	if d.namespace != "" {
		args = append(args, "--namespace", d.namespace)
	}
	if d.kubeContext != "" {
		args = append(args, "--kube-context", d.kubeContext)
	}
	cmd := exec.Command(os.Getenv("HELM_BIN"), args...)
	debugPrint("Executing %s", strings.Join(cmd.Args, " "))
	defer func() {
		_ = f.Close()
	}()
	cmd.Stdout = f
	return cmd.Run()
}

func extractManifestFromHelmUpgradeDryRunOutput(s []byte, noHooks bool) []byte {
	if len(s) == 0 {
		return s
	}

	var (
		hooks    []byte
		manifest []byte
	)

	i := bytes.Index(s, []byte("HOOKS:"))
	if i > -1 {
		hooks = s[i:]
	}

	j := bytes.Index(hooks, []byte("MANIFEST:"))
	if j > -1 {
		manifest = hooks[j:]
		hooks = hooks[:j]
	}

	k := bytes.Index(manifest, []byte("\nNOTES:"))

	if k > -1 {
		manifest = manifest[:k+1]
	}

	if noHooks {
		hooks = nil
	} else {
		a := bytes.Index(hooks, []byte("---"))
		if a > -1 {
			hooks = hooks[a:]
		} else {
			hooks = nil
		}
	}

	a := bytes.Index(manifest, []byte("---"))
	if a > -1 {
		manifest = manifest[a:]
	}

	r := []byte{}
	r = append(r, manifest...)
	r = append(r, hooks...)

	return r
}

// ociPullProgressRE matches Helm's OCI chart pull progress lines that Helm writes
// to stdout before the rendered manifests when the chart, or one of its subcharts,
// is pulled from an OCI registry.
//
// The lines reported in the wild are "Pulled: ..." and "Digest: ...";
// "Pulling: ..." is matched defensively too.
//
// These lines are emitted at the start of a line and are not valid Kubernetes
// manifests, so they are safe to strip. Top-level manifest keys are
// apiVersion/kind/metadata/spec and never "Pulled", "Digest" or "Pulling";
// any homonymous keys inside a manifest are indented and therefore not matched.
//
// See https://github.com/databus23/helm-diff/issues/1040
var ociPullProgressRE = regexp.MustCompile(`(?m)^(?:Pulled|Digest|Pulling):[^\n]*\n?`)

// stripOCIPullProgress removes Helm's OCI chart pull progress output that leaks
// into the rendered manifest buffer when the chart (or a subchart) is pulled
// from an OCI registry.
//
// Without this, the progress lines (e.g. "Pulled: ...", "Digest: ...") are
// parsed as a YAML document lacking a Kind and break the downstream three-way
// merge / kubeclient.Build():
//
//	unable to decode "": Object 'Kind' is missing in '{"Digest":"...","Pulled":"..."}'
//
// See https://github.com/databus23/helm-diff/issues/1040
func stripOCIPullProgress(s []byte) []byte {
	return ociPullProgressRE.ReplaceAll(s, []byte(""))
}

// serverSideFlags returns the --server-side flag(s) to forward to helm.
//
// The flag is Helm v4 only:
//   - `helm upgrade` registers it as a string accepting "true", "false", "auto".
//   - `helm template` registers it as a bool (default true); "auto" is rejected.
//
// The flag has no effect on manifest rendering in dry-run mode — both
// `helm template` and `helm upgrade --dry-run` bail out before the apply
// step where ServerSideApply is actually consulted. It is forwarded purely
// for semantic correctness and so that helm-diff can be used as a drop-in
// wrapper around `helm upgrade` with the same flags.
func serverSideFlags(isHelmV4 bool, useUpgradeDryRun bool, serverSide string) []string {
	if !isHelmV4 {
		return nil
	}
	switch {
	case useUpgradeDryRun:
		// `helm upgrade`: forward all values including "auto".
		return []string{"--server-side=" + serverSide}
	case serverSide == envTrue || serverSide == envFalse:
		// `helm template`: forward only bool-compatible values.
		// "auto" is skipped — template's default (true) is reasonable.
		return []string{"--server-side=" + serverSide}
	default:
		return nil
	}
}
