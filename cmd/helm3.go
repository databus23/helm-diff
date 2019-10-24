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
	if !d.resetValues {
		if d.reuseValues {
			tmpfile, err := ioutil.TempFile("", "existing-values")
			if err != nil {
				return nil, err
			}
			defer os.Remove(tmpfile.Name())
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
	}

	args := []string{"template", d.release, d.chart}
	args = append(args, flags...)
	cmd := exec.Command(os.Getenv("HELM_BIN"), args...)
	return outputWithRichError(cmd)
}

func (d *diffCmd) existingValues(f *os.File) error {
	cmd := exec.Command(os.Getenv("HELM_BIN"), "get", "values", d.release, "--all")
	debugPrint("Executing %s", strings.Join(cmd.Args, " "))
	defer f.Close()
	cmd.Stdout = f
	return cmd.Run()
}
