package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"k8s.io/helm/cmd/helm/helmpath"
	"k8s.io/helm/cmd/helm/strvals"
	"k8s.io/helm/pkg/downloader"
	"k8s.io/helm/pkg/helm"

	"github.com/databus23/helm-diff/manifest"
)

const globalUsage = `
Show a diff explaing what a helm upgrade would change.

This fetches the currently deployed version of a release
and compares it to a local chart plus values.
This can be used visualize what changes a helm upgrade will
perform.
`

type diffCmd struct {
	release string
	chart   string
	//	out     io.Writer
	client helm.Interface
	//	version int32
	valueFiles valueFiles
	values     []string
}

type valueFiles []string

func (v *valueFiles) String() string {
	return fmt.Sprint(*v)
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

func main() {

	diff := diffCmd{}

	cmd := &cobra.Command{
		Use:   "diff [flags] [RELEASE] [CHART]",
		Short: "Show manifest differences",
		Long:  globalUsage,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkArgsLength(len(args), "release name", "chart path"); err != nil {
				return err
			}

			diff.release = args[0]
			diff.chart = args[1]
			if diff.client == nil {
				diff.client = helm.NewClient(helm.Host(os.Getenv("TILLER_HOST")))
			}
			return diff.run()
		},
	}
	f := cmd.Flags()
	f.VarP(&diff.valueFiles, "values", "f", "specify values in a YAML file (can specify multiple)")
	f.StringArrayVar(&diff.values, "set", []string{}, "set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func (d *diffCmd) run() error {
	chartPath, err := locateChartPath(d.chart, "", false, "")
	if err != nil {
		return err
	}

	rawVals, err := d.vals()
	if err != nil {
		return err
	}

	releaseResponse, err := d.client.ReleaseContent(d.release)

	if err != nil {
		return prettyError(err)
	}

	upgradeResponse, err := d.client.UpdateRelease(
		d.release,
		chartPath,
		helm.UpdateValueOverrides(rawVals),
		helm.UpgradeDryRun(true),
	)
	if err != nil {
		return prettyError(err)
	}

	currentSpecs := manifest.Parse(releaseResponse.Release.Manifest)
	newSpecs := manifest.Parse(upgradeResponse.Release.Manifest)

	diffManifests(currentSpecs, newSpecs, os.Stdout)

	return nil
}

func (d *diffCmd) vals() ([]byte, error) {
	base := map[string]interface{}{}

	// User specified a values files via -f/--values
	for _, filePath := range d.valueFiles {
		currentMap := map[string]interface{}{}
		bytes, err := ioutil.ReadFile(filePath)
		if err != nil {
			return []byte{}, err
		}

		if err := yaml.Unmarshal(bytes, &currentMap); err != nil {
			return []byte{}, fmt.Errorf("failed to parse %s: %s", filePath, err)
		}
		// Merge with the previous map
		base = mergeValues(base, currentMap)
	}

	// User specified a value via --set
	for _, value := range d.values {
		if err := strvals.ParseInto(value, base); err != nil {
			return []byte{}, fmt.Errorf("failed parsing --set data: %s", err)
		}
	}

	return yaml.Marshal(base)
}

func prettyError(err error) error {
	if err == nil {
		return nil
	}
	// This is ridiculous. Why is 'grpc.rpcError' not exported? The least they
	// could do is throw an interface on the lib that would let us get back
	// the desc. Instead, we have to pass ALL errors through this.
	return errors.New(grpc.ErrorDesc(err))
}

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

func locateChartPath(name, version string, verify bool, keyring string) (string, error) {
	name = strings.TrimSpace(name)
	version = strings.TrimSpace(version)
	if fi, err := os.Stat(name); err == nil {
		abs, err := filepath.Abs(name)
		if err != nil {
			return abs, err
		}
		if verify {
			if fi.IsDir() {
				return "", errors.New("cannot verify a directory")
			}
			if _, err := downloader.VerifyChart(abs, keyring); err != nil {
				return "", err
			}
		}
		return abs, nil
	}
	if filepath.IsAbs(name) || strings.HasPrefix(name, ".") {
		return name, fmt.Errorf("path %q not found", name)
	}

	crepo := filepath.Join(helmpath.Home(homePath()).Repository(), name)
	if _, err := os.Stat(crepo); err == nil {
		return filepath.Abs(crepo)
	}

	dl := downloader.ChartDownloader{
		HelmHome: helmpath.Home(homePath()),
		Out:      os.Stdout,
		Keyring:  keyring,
	}
	if verify {
		dl.Verify = downloader.VerifyAlways
	}

	filename, _, err := dl.DownloadTo(name, version, ".")
	if err == nil {
		lname, err := filepath.Abs(filename)
		if err != nil {
			return filename, err
		}
		return lname, nil
	}

	return filename, fmt.Errorf("file %q not found", name)
}

func homePath() string {
	return os.Getenv("HELM_HOME")
}

// Merges source and destination map, preferring values from the source map
func mergeValues(dest map[string]interface{}, src map[string]interface{}) map[string]interface{} {
	for k, v := range src {
		// If the key doesn't exist already, then just set the key to that value
		if _, exists := dest[k]; !exists {
			dest[k] = v
			continue
		}
		nextMap, ok := v.(map[string]interface{})
		// If it isn't another map, overwrite the value
		if !ok {
			dest[k] = v
			continue
		}
		// If the key doesn't exist already, then just set the key to that value
		if _, exists := dest[k]; !exists {
			dest[k] = nextMap
			continue
		}
		// Edge case: If the key exists in the destination, but isn't a map
		destMap, isMap := dest[k].(map[string]interface{})
		// If the source map has a map for this key, prefer it
		if !isMap {
			dest[k] = v
			continue
		}
		// If we got to this point, it is a map in both, so merge them
		dest[k] = mergeValues(destMap, nextMap)
	}
	return dest
}
