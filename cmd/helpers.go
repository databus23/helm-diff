package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	flag "github.com/spf13/pflag"
	"k8s.io/client-go/util/homedir"
	"k8s.io/helm/pkg/helm"
	helm_env "k8s.io/helm/pkg/helm/environment"
	"k8s.io/helm/pkg/tlsutil"
)

const (
	tlsCaCertDefault = "$HELM_HOME/ca.pem"
	tlsCertDefault   = "$HELM_HOME/cert.pem"
	tlsKeyDefault    = "$HELM_HOME/key.pem"

	helm2TestSuccessHook = "test-success"
	helm3TestHook        = "test"
)

var (
	settings helm_env.EnvSettings
	// DefaultHelmHome to hold default home path of .helm dir
	DefaultHelmHome = filepath.Join(homedir.HomeDir(), ".helm")
)

func isHelm3() bool {
	return os.Getenv("TILLER_HOST") == ""
}

func addCommonCmdOptions(f *flag.FlagSet) {
	settings.AddFlagsTLS(f)
	settings.InitTLS(f)

	f.StringVar((*string)(&settings.Home), "home", DefaultHelmHome, "location of your Helm config. Overrides $HELM_HOME")
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
