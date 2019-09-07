package cmd

import (
	"fmt"
	"github.com/databus23/helm-diff/diff"
	"github.com/databus23/helm-diff/manifest"
	"github.com/pkg/errors"
	"helm.sh/helm/pkg/kube"
	"log"
	"os"
	"strings"
	"sync"

	"helm.sh/helm/pkg/action"
	"helm.sh/helm/pkg/chart"
	"helm.sh/helm/pkg/chart/loader"
	"helm.sh/helm/pkg/cli"
	"helm.sh/helm/pkg/cli/values"
	"helm.sh/helm/pkg/getter"
	helm3release "helm.sh/helm/pkg/release"
	"helm.sh/helm/pkg/storage"
	"helm.sh/helm/pkg/storage/driver"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	// Import to initialize client auth plugins.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	config      genericclioptions.RESTClientGetter
	configOnce  sync.Once
	envSettings *cli.EnvSettings
)

func init() {
	envSettings = cli.New()
}

// Helm3Client is the client for interacting with Helm3 releases
type Helm3Client struct {
	conf     *action.Configuration
	settings *cli.EnvSettings
}

func helm3Run(d *diffCmd, chartPath string) error {
	name := d.release
	chart := d.chart

	helm3 := NewHelm3()

	releaseResponse, err := helm3.Get(name, 0)

	var newInstall bool
	if err != nil && strings.Contains(err.Error(), fmt.Sprintf("release: %q not found", d.release)) {
		if d.allowUnreleased {
			fmt.Printf("********************\n\n\tRelease was not present in Helm.  Diff will show entire contents as new.\n\n********************\n")
			newInstall = true
			err = nil
		} else {
			fmt.Printf("********************\n\n\tRelease was not present in Helm.  Include the `--allow-unreleased` to perform diff without exiting in error.\n\n********************\n")
		}
	}

	if err != nil {
		return prettyError(fmt.Errorf("get: %v", err))
	}

	var currentSpecs, newSpecs map[string]*manifest.MappingResult
	valOpts := &values.Options{
		ValueFiles:   d.valueFiles,
		Values:       d.values,
		StringValues: d.stringValues,
	}
	if newInstall {
		installResponse, err := helm3.Install(d.release, chart, valOpts)
		if err != nil {
			return prettyError(fmt.Errorf("install: %v", err))
		}

		currentSpecs = make(map[string]*manifest.MappingResult)
		newSpecs = manifest.Parse(installResponse.Manifest, installResponse.Namespace)
	} else {
		upgradeResponse, err := helm3.Upgrade(d.release, chart, valOpts)
		if err != nil {
			return prettyError(fmt.Errorf("upgrade: %v", err))
		}

		if d.noHooks {
			currentSpecs = manifest.Parse(releaseResponse.Manifest, releaseResponse.Namespace)
			newSpecs = manifest.Parse(upgradeResponse.Manifest, upgradeResponse.Namespace)
		} else {
			currentSpecs = ParseRelease(releaseResponse, d.includeTests)
			newSpecs = ParseRelease(upgradeResponse, d.includeTests)
		}
	}

	seenAnyChanges := diff.Manifests(currentSpecs, newSpecs, d.suppressedKinds, d.outputContext, os.Stdout)

	if d.detailedExitCode && seenAnyChanges {
		return Error{
			error: errors.New("identified at least one change, exiting with non-zero exit code (detailed-exitcode parameter enabled)"),
			Code:  2,
		}
	}

	return nil
}

// ParseRelease parses Helm v3 release to obtain MappingResults
func ParseRelease(release *helm3release.Release, includeTests bool) map[string]*manifest.MappingResult {
	man := release.Manifest
	for _, hook := range release.Hooks {
		if !includeTests && isTestHook(hook.Events) {
			continue
		}

		man += "\n---\n"
		man += fmt.Sprintf("# Source: %s\n", hook.Path)
		man += hook.Manifest
	}
	return manifest.Parse(man, release.Namespace)
}

func isTestHook(hookEvents []helm3release.HookEvent) bool {
	for _, event := range hookEvents {
		if event == helm3release.HookTest {
			return true
		}
	}

	return false
}

// NewHelm3 returns Helm3 client for use within helm-diff
func NewHelm3() *Helm3Client {
	conf := &action.Configuration{}
	initActionConfig(conf, false)
	return &Helm3Client{
		conf:     conf,
		settings: envSettings,
	}
}

// Get returns the named release
func (helm3 *Helm3Client) Get(name string, version int) (*helm3release.Release, error) {
	if version <= 0 {
		return helm3.conf.Releases.Last(name)
	}

	return helm3.conf.Releases.Get(name, version)
}

// Upgrade returns the named release after an upgrade
func (helm3 *Helm3Client) Upgrade(name, chart string, valueOpts *values.Options) (*helm3release.Release, error) {
	conf := helm3.conf

	settings := helm3.settings

	client := action.NewUpgrade(conf)
	client.DryRun = true

	getters := getter.All(settings)
	vals, err := valueOpts.MergeValues(getters)
	if err != nil {
		return nil, fmt.Errorf("merge values: %v", err)
	}

	chartRequested, err := helm3.loadChart(chart, client.ChartPathOptions.LocateChart, settings)
	if err != nil {
		return nil, fmt.Errorf("load chart: %v", err)
	}

	r, err := client.Run(name, chartRequested, vals)
	if err != nil {
		return nil, fmt.Errorf("run: %v", err)
	}
	return r, nil
}

// Install returns the simulated release after installing
func (helm3 *Helm3Client) Install(name, chart string, valueOpts *values.Options) (*helm3release.Release, error) {
	conf := helm3.conf

	args := []string{name, chart}

	settings := helm3.settings

	client := action.NewInstall(conf)
	client.DryRun = true

	name, chart, err := client.NameAndChart(args)
	if err != nil {
		return nil, err
	}
	client.ReleaseName = name

	getters := getter.All(settings)
	vals, err := valueOpts.MergeValues(getters)
	if err != nil {
		return nil, err
	}

	chartRequested, err := helm3.loadChart(chart, client.ChartPathOptions.LocateChart, settings)
	if err != nil {
		return nil, err
	}

	client.Namespace = helm3.getNamespace()
	return client.Run(chartRequested, vals)
}

//func (helm3 *Helm3Client) kubeConfig() genericclioptions.RESTClientGetter {
//	if helm3.config == nil {
//		settings := helm3.settings
//		helm3.config = kube.GetConfig(settings.KubeConfig, settings.KubeContext, settings.Namespace)
//	}
//	return helm3.config
//}
//
func (helm3 *Helm3Client) getNamespace() string {
	if helm3.settings.Namespace != "" {
		return helm3.settings.Namespace
	}

	//if ns, _, err := cli.kubeConfig().ToRawKubeConfigLoader().Namespace(); err == nil {
	//	return ns
	//}
	return "default"
}

func (helm3 *Helm3Client) loadChart(chart string, locateChart func(name string, settings *cli.EnvSettings) (string, error), settings *cli.EnvSettings) (*chart.Chart, error) {
	chartPath, err := locateChart(chart, settings)
	if err != nil {
		return nil, fmt.Errorf("locate chart: %v", err)
	}

	debug("CHART PATH: %s\n", chartPath)

	// Check chart dependencies to make sure all are present in /charts
	chartRequested, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("load: %v", err)
	}

	validInstallableChart, checkErr := isChartInstallable(chartRequested)
	if !validInstallableChart {
		return nil, fmt.Errorf("invalid chart: checkErr=%v, err=%v", checkErr, err)
	}

	return chartRequested, nil
}

// isChartInstallable validates if a chart can be installed
//
// Application chart type is only installable
func isChartInstallable(ch *chart.Chart) (bool, error) {
	switch ch.Metadata.Type {
	case "", "application":
		return true, nil
	}
	return false, errors.Errorf("%s charts are not installable", ch.Metadata.Type)
}

func debug(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg, args...)
}

func initActionConfig(actionConfig *action.Configuration, allNamespaces bool) {
	kc := kube.New(kubeConfig())
	kc.Log = debug

	clientset, err := kc.Factory.KubernetesClientSet()
	if err != nil {
		// TODO return error
		log.Fatal(err)
	}
	var namespace string
	if !allNamespaces {
		namespace = getNamespace()
	}

	var store *storage.Storage
	switch os.Getenv("HELM_DRIVER") {
	case "secret", "secrets", "":
		d := driver.NewSecrets(clientset.CoreV1().Secrets(namespace))
		d.Log = debug
		store = storage.Init(d)
	case "configmap", "configmaps":
		d := driver.NewConfigMaps(clientset.CoreV1().ConfigMaps(namespace))
		d.Log = debug
		store = storage.Init(d)
	case "memory":
		d := driver.NewMemory()
		store = storage.Init(d)
	default:
		// Not sure what to do here.
		panic("Unknown driver in HELM_DRIVER: " + os.Getenv("HELM_DRIVER"))
	}

	actionConfig.RESTClientGetter = kubeConfig()
	actionConfig.KubeClient = kc
	actionConfig.Releases = store
	actionConfig.Log = debug
}

func getNamespace() string {
	if envSettings.Namespace != "" {
		return envSettings.Namespace
	}

	if ns, _, err := kubeConfig().ToRawKubeConfigLoader().Namespace(); err == nil {
		return ns
	}
	return "default"
}

func kubeConfig() genericclioptions.RESTClientGetter {
	configOnce.Do(func() {
		config = kube.GetConfig(settings.KubeConfig, settings.KubeContext, envSettings.Namespace)
	})
	return config
}
