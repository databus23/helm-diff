package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/util/homedir"

	flag "github.com/spf13/pflag"
	"k8s.io/helm/pkg/helm"
	helm_env "k8s.io/helm/pkg/helm/environment"
	"k8s.io/helm/pkg/tlsutil"
)

const (
	tlsCaCertDefault = "$HELM_HOME/ca.pem"
	tlsCertDefault   = "$HELM_HOME/cert.pem"
	tlsKeyDefault    = "$HELM_HOME/key.pem"
)

var (
	settings        helm_env.EnvSettings
	DefaultHelmHome = filepath.Join(homedir.HomeDir(), ".helm")

	tlsCaCertFile string // path to TLS CA certificate file
	tlsCertFile   string // path to TLS certificate file
	tlsKeyFile    string // path to TLS key file
	tlsVerify     bool   // enable TLS and verify remote certificates
	tlsEnable     bool   // enable TLS
)

func addCommonCmdOptions(f *flag.FlagSet) {
	f.StringVar(&tlsCaCertFile, "tls-ca-cert", tlsCaCertDefault, "path to TLS CA certificate file")
	f.StringVar(&tlsCertFile, "tls-cert", tlsCertDefault, "path to TLS certificate file")
	f.StringVar(&tlsKeyFile, "tls-key", tlsKeyDefault, "path to TLS key file")
	f.BoolVar(&tlsVerify, "tls-verify", false, "enable TLS for request and verify remote")
	f.BoolVar(&tlsEnable, "tls", false, "enable TLS for request")

	f.StringVar((*string)(&settings.Home), "home", DefaultHelmHome, "location of your Helm config. Overrides $HELM_HOME")
}

func createHelmClient() helm.Interface {
	options := []helm.Option{helm.Host(os.Getenv("TILLER_HOST")), helm.ConnectTimeout(int64(30))}

	if tlsVerify || tlsEnable {
		if tlsCaCertFile == "" {
			tlsCaCertFile = settings.Home.TLSCaCert()
		}
		if tlsCertFile == "" {
			tlsCertFile = settings.Home.TLSCert()
		}
		if tlsKeyFile == "" {
			tlsKeyFile = settings.Home.TLSKey()
		}

		tlsopts := tlsutil.Options{KeyFile: tlsKeyFile, CertFile: tlsCertFile, InsecureSkipVerify: true}
		if tlsVerify {
			tlsopts.CaCertFile = tlsCaCertFile
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
	tlsCaCertFile = os.ExpandEnv(tlsCaCertFile)
	tlsCertFile = os.ExpandEnv(tlsCertFile)
	tlsKeyFile = os.ExpandEnv(tlsKeyFile)
}
