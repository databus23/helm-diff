package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	flag "github.com/spf13/pflag"
	"helm.sh/helm/pkg/action"
	"helm.sh/helm/pkg/cli"
	"helm.sh/helm/pkg/cli/values"
	"helm.sh/helm/pkg/kube"
	rspb "helm.sh/helm/pkg/release"
	"helm.sh/helm/pkg/storage"
	"helm.sh/helm/pkg/storage/driver"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/util/homedir"
	"k8s.io/helm/pkg/helm"
	helm_env "k8s.io/helm/pkg/helm/environment"
	rls "k8s.io/helm/pkg/proto/hapi/services"
	"k8s.io/helm/pkg/tlsutil"

	"github.com/databus23/helm-diff/manifest"
)

const (
	tlsCaCertDefault = "$HELM_HOME/ca.pem"
	tlsCertDefault   = "$HELM_HOME/cert.pem"
	tlsKeyDefault    = "$HELM_HOME/key.pem"
)

var (
	settings helm_env.EnvSettings
	// DefaultHelmHome to hold default home path of .helm dir
	DefaultHelmHome = filepath.Join(homedir.HomeDir(), ".helm")
	settingsV3      *cli.EnvSettings
	config          genericclioptions.RESTClientGetter
	configOnce      sync.Once
)

type clientHolder struct {
	client       helm.Interface
	actionConfig action.Configuration
}

func addCommonCmdOptions(f *flag.FlagSet) {
	if isHelm3() {
		settingsV3 = cli.New()
		settingsV3.AddFlags(f)
		settingsV3.Init(f)
	} else {
		settings.AddFlagsTLS(f)
		settings.InitTLS(f)

		f.StringVar((*string)(&settings.Home), "home", DefaultHelmHome, "location of your Helm config. Overrides $HELM_HOME")
	}
}

func (cmd *clientHolder) deployedRelease(release string) (releaseResponse manifest.ReleaseResponse, err error) {
	if isHelm3() {
		var response *rspb.Release
		response, err = cmd.actionConfig.Releases.Deployed(release)
		releaseResponse.ReleaseV3 = response
	} else {
		var response *rls.GetReleaseContentResponse
		response, err = cmd.client.ReleaseContent(release)
		releaseResponse.Release = response.Release
	}
	return
}

func (cmd *clientHolder) deployedReleaseRevision(release string, version int) (releaseResponse manifest.ReleaseResponse, err error) {
	if isHelm3() {
		var response *rspb.Release
		response, err = cmd.actionConfig.Releases.Get(release, version)
		releaseResponse.ReleaseV3 = response
	} else {
		var response *rls.GetReleaseContentResponse
		response, err = cmd.client.ReleaseContent(release, helm.ContentReleaseVersion(int32(version)))
		releaseResponse.Release = response.Release
	}
	return
}

func (cmd *clientHolder) init() {
	if isHelm3() {
		if cmd.actionConfig.Releases == nil {
			initActionConfig(&cmd.actionConfig)
		}
	} else {
		if cmd.client == nil {
			cmd.client = createHelmClient()
		}
	}
}

func isHelm3() bool {
	return os.Getenv("TILLER_HOST") == ""
}

func addValueOptionsFlags(f *flag.FlagSet, v *values.Options) {
	f.StringSliceVarP(&v.ValueFiles, "values", "f", []string{}, "specify values in a YAML file or a URL(can specify multiple)")
	f.StringArrayVar(&v.Values, "set", []string{}, "set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringArrayVar(&v.StringValues, "set-string", []string{}, "set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
}

func initActionConfig(actionConfig *action.Configuration) {
	kc := kube.New(kubeConfig())
	kc.Log = debugV3

	clientset, err := kc.Factory.KubernetesClientSet()
	if err != nil {
		// TODO return error
		log.Fatal(err)
	}
	namespace := getNamespace()

	var store *storage.Storage
	switch os.Getenv("HELM_DRIVER") {
	case "secret", "secrets", "":
		d := driver.NewSecrets(clientset.CoreV1().Secrets(namespace))
		d.Log = debugV3
		store = storage.Init(d)
	case "configmap", "configmaps":
		d := driver.NewConfigMaps(clientset.CoreV1().ConfigMaps(namespace))
		d.Log = debugV3
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
	actionConfig.Log = debugV3
}

func getNamespace() string {
	if settingsV3.Namespace != "" {
		return settingsV3.Namespace
	}

	if ns, _, err := kubeConfig().ToRawKubeConfigLoader().Namespace(); err == nil {
		return ns
	}
	return "default"
}

func kubeConfig() genericclioptions.RESTClientGetter {
	configOnce.Do(func() {
		config = kube.GetConfig(settingsV3.KubeConfig, settingsV3.KubeContext, settingsV3.Namespace)
	})
	return config
}

func debugV3(format string, v ...interface{}) {
	if settingsV3.Debug {
		format = fmt.Sprintf("[debug] %s\n", format)
		log.Output(2, fmt.Sprintf(format, v...))
	}
}

func createHelmClient() helm.Interface {
	options := []helm.Option{helm.Host(os.Getenv("TILLER_HOST")), helm.ConnectTimeout(int64(30))}

	if settings.TLSVerify || settings.TLSEnable {
		tlsopts := tlsutil.Options{
			ServerName:         settings.TLSServerName,
			KeyFile:            settings.TLSKeyFile,
			CertFile:           settings.TLSCertFile,
			InsecureSkipVerify: true,
		}

		if settings.TLSVerify {
			tlsopts.CaCertFile = settings.TLSCaCertFile
			tlsopts.InsecureSkipVerify = false
		}

		tlscfg, err := tlsutil.ClientConfig(tlsopts)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}

		options = append(options, helm.WithTLS(tlscfg))
	}

	return helm.NewClient(options...)
}

func expandTLSPaths() {
	settings.TLSCaCertFile = os.ExpandEnv(settings.TLSCaCertFile)
	settings.TLSCertFile = os.ExpandEnv(settings.TLSCertFile)
	settings.TLSKeyFile = os.ExpandEnv(settings.TLSKeyFile)
}
