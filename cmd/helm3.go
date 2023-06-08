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

	"github.com/Masterminds/semver"
)

var (
	helmVersionRE  = regexp.MustCompile(`Version:\s*"([^"]+)"`)
	minHelmVersion = semver.MustParse("v3.1.0-rc.1")
)

func compatibleHelm3Version() error {
	cmd := exec.Command(os.Getenv("HELM_BIN"), "version")
	debugPrint("Executing %s", strings.Join(cmd.Args, " "))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Failed to run `%s version`: %v", os.Getenv("HELM_BIN"), err)
	}
	versionOutput := string(output)

	matches := helmVersionRE.FindStringSubmatch(versionOutput)
	if matches == nil {
		return fmt.Errorf("Failed to find version in output %#v", versionOutput)
	}
	helmVersion, err := semver.NewVersion(matches[1])
	if err != nil {
		return fmt.Errorf("Failed to parse version %#v: %v", matches[1], err)
	}

	if minHelmVersion.GreaterThan(helmVersion) {
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
	// Helm automatically enable --reuse-values when there's no --set, --set-string, --set-values, --set-file present.
	// Let's simulate that in helm-diff.
	// See https://medium.com/@kcatstack/understand-helm-upgrade-flags-reset-values-reuse-values-6e58ac8f127e
	shouldDefaultReusingValues := isUpgrade && len(d.values) == 0 && len(d.stringValues) == 0 && len(d.valueFiles) == 0 && len(d.fileValues) == 0
	if (d.reuseValues || shouldDefaultReusingValues) && !d.resetValues && !d.dryRun {
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

	var (
		subcmd string
		filter func([]byte) []byte
	)

	if d.useUpgradeDryRun {
		if d.dryRun {
			return nil, fmt.Errorf("`diff upgrade --dry-run` conflicts with HELM_DIFF_USE_UPGRADE_DRY_RUN_AS_TEMPLATE. Either remove --dry-run to enable cluster access, or unset HELM_DIFF_USE_UPGRADE_DRY_RUN_AS_TEMPLATE to make cluster access unnecessary")
		}

		if d.isAllowUnreleased() {
			// Otherwise you get the following error when this is a diff for a new install
			//   Error: UPGRADE FAILED: "$RELEASE_NAME" has no deployed releases
			flags = append(flags, "--install")
		}

		flags = append(flags, "--dry-run")
		subcmd = "upgrade"
		filter = func(s []byte) []byte {
			return extractManifestFromHelmUpgradeDryRunOutput(s, d.noHooks)
		}
	} else {
		if !d.disableValidation && !d.dryRun {
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
