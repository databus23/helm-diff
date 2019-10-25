package cmd

import (
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

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
	args := []string{"get", release, "--template", "{{.Release.Chart.Name}}"}
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

func (d *diffCmd) template() ([]byte, error) {
	flags := []string{}
	if d.devel {
		flags = append(flags, "--devel")
	}
	if d.noHooks {
		flags = append(flags, "--no-hooks")
	}
	if d.chartVersion != "" {
		flags = append(flags, "--version", d.chartVersion)
	}
	if d.namespace != "" {
		flags = append(flags, "--namespace", d.namespace)
	}
	// Helm automatically enable --reuse-values when there's no --set, --set-string, --set-values, --set-file present.
	// Let's simulate that in helm-diff.
	// See https://medium.com/@kcatstack/understand-helm-upgrade-flags-reset-values-reuse-values-6e58ac8f127e
	shouldDefaultReusingValues := len(d.values) == 0 && len(d.stringValues) == 0 && len(d.valueFiles) == 0 && len(d.fileValues) == 0
	if (d.reuseValues || shouldDefaultReusingValues) && !d.resetValues {
		tmpfile, err := ioutil.TempFile("", "existing-values")
		if err != nil {
			return nil, err
		}
		defer os.Remove(tmpfile.Name())
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
		flags = append(flags, "--values", valueFile)
	}
	for _, fileValue := range d.fileValues {
		flags = append(flags, "--set-file", fileValue)
	}

	//This is a workaround until https://github.com/helm/helm/pull/6729 is released
	for _, apiVersion := range strings.Split(os.Getenv("HELM_TEMPLATE_API_VERSIONS"), ",") {
		if apiVersion != "" {
			flags = append(flags, "--api-versions", strings.TrimSpace(apiVersion))
		}
	}

	args := []string{"template", d.release, d.chart}
	args = append(args, flags...)
	cmd := exec.Command(os.Getenv("HELM_BIN"), args...)
	return outputWithRichError(cmd)
}

func (d *diffCmd) writeExistingValues(f *os.File) error {
	cmd := exec.Command(os.Getenv("HELM_BIN"), "get", "values", d.release, "--all", "--output", "yaml")
	debugPrint("Executing %s", strings.Join(cmd.Args, " "))
	defer f.Close()
	cmd.Stdout = f
	return cmd.Run()
}
