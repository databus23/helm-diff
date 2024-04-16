package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
)

var (
	helmVersionRE  = regexp.MustCompile(`Version:\s*"([^"]+)"`)
	minHelmVersion = semver.MustParse("v3.1.0-rc.1")
	// See https://github.com/helm/helm/pull/9426
	minHelmVersionWithDryRunLookupSupport = semver.MustParse("v3.13.0")
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

func compatibleHelm3Version() error {
	if isCompatible, err := isHelmVersionAtLeast(minHelmVersion); err != nil {
		return err
	} else if !isCompatible {
		return fmt.Errorf("helm diff upgrade requires at least helm version %s", minHelmVersion.String())
	}
	return nil
}

func getRelease(release, namespace string) ([]byte, error) {
	args := []string{"get", "manifest", release}
	if namespace != "" {
		args = append(args, "--namespace", namespace)
	}
	cmd := exec.Command(os.Getenv("HELM_BIN"), args...)
	return outputWithRichError(cmd)
}

func getHooks(release, namespace string) ([]byte, error) {
	args := []string{"get", "hooks", release}
	if namespace != "" {
		args = append(args, "--namespace", namespace)
	}
	cmd := exec.Command(os.Getenv("HELM_BIN"), args...)
	return outputWithRichError(cmd)
}

func getRevision(release string, revision int, namespace string) ([]byte, error) {
	args := []string{"get", "manifest", release, "--revision", strconv.Itoa(revision)}
	if namespace != "" {
		args = append(args, "--namespace", namespace)
	}
	cmd := exec.Command(os.Getenv("HELM_BIN"), args...)
	return outputWithRichError(cmd)
}

func getChart(release, namespace string) (string, error) {
	args := []string{"get", "all", release, "--template", "{{.Release.Chart.Name}}"}
	if namespace != "" {
		args = append(args, "--namespace", namespace)
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
	if d.chartVersion != "" {
		flags = append(flags, "--version", d.chartVersion)
	}
	if d.chartRepo != "" {
		flags = append(flags, "--repo", d.chartRepo)
	}
	if d.namespace != "" {
		flags = append(flags, "--namespace", d.namespace)
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
	if (d.reuseValues || shouldDefaultReusingValues) && !d.resetValues && d.clusterAccessAllowed() {
		tmpfile, err := os.CreateTemp("", "existing-values")
		if err != nil {
			return nil, err
		}
		defer func() {
			_ = os.Remove(tmpfile.Name())
		}()
		if err := d.writeExistingValues(tmpfile); err != nil {
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
			flags = append(flags, "--validate")
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
			// However, which dry-run mode to use is still not clear.
			//
			// For compatibility with the old and new helm-diff options,
			// old and new helm, we assume that the user wants to use the older `helm template --dry-run=client` mode
			// if helm-diff has been invoked with any of the following flags:
			//
			// * no dry-run flags (to be consistent with helm-template)
			// * --dry-run
			// * --dry-run=""
			// * --dry-run=client
			//
			// and the newer `helm template --dry-run=server` mode when invoked with:
			//
			// * --dry-run=server
			//
			// Any other values should result in errors.
			//
			// See the fllowing link for more details:
			// - https://github.com/databus23/helm-diff/pull/458
			// - https://github.com/helm/helm/pull/9426#issuecomment-1501005666
			if d.dryRunMode == "server" {
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

		subcmd = "template"

		filter = func(s []byte) []byte {
			return s
		}
	}

	args := []string{subcmd, d.release, d.chart}
	args = append(args, flags...)

	cmd := exec.Command(os.Getenv("HELM_BIN"), args...)
	out, err := outputWithRichError(cmd)
	return filter(out), err
}

func (d *diffCmd) writeExistingValues(f *os.File) error {
	cmd := exec.Command(os.Getenv("HELM_BIN"), "get", "values", d.release, "--all", "--output", "yaml")
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

	i := bytes.Index(s, []byte("HOOKS:"))
	hooks := s[i:]

	j := bytes.Index(hooks, []byte("MANIFEST:"))

	manifest := hooks[j:]
	hooks = hooks[:j]

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
