package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path"
	"strconv"
	"syscall"

	"github.com/golang/glog"
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/shawn-hurley/osb-broker-k8s-lib/middleware"
	clientset "k8s.io/client-go/kubernetes"
	clientrest "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/pmorie/osb-broker-lib/pkg/metrics"
	"github.com/pmorie/osb-broker-lib/pkg/rest"
	"github.com/pmorie/osb-broker-lib/pkg/server"

	"cloudfront-broker/pkg/broker"
)

var options struct {
	broker.Options

	Port                 int
	Insecure             bool
	TLSCert              string
	TLSKey               string
	TLSCertFile          string
	TLSKeyFile           string
	AuthenticateK8SToken bool
	KubeConfig           string
	Version              bool
}

// Variables used to output version and build time information
var (
	Version    string
	GitCommit  string
	GoVersion  string
	Built      string
	OSBVersion string
)

func init() {
	flag.IntVar(&options.Port, "port", 5443, "use '--port' option to specify the port for broker to listen on")
	flag.BoolVar(&options.Insecure, "insecure", false, "use --insecure to use HTTP vs HTTPS.")
	flag.StringVar(&options.TLSCertFile, "tls-cert-file", "", "File containing the default x509 Certificate for HTTPS. (CA cert, if any, concatenated after server cert).")
	flag.StringVar(&options.TLSKeyFile, "tls-private-key-file", "", "File containing the default x509 private key matching --tls-cert-file.")
	flag.StringVar(&options.TLSCert, "tlsCert", "", "base-64 encoded PEM block to use as the certificate for TLS. If '--tlsCert' is used, then '--tlsKey' must also be used.")
	flag.StringVar(&options.TLSKey, "tlsKey", "", "base-64 encoded PEM block to use as the private key matching the TLS certificate.")
	flag.BoolVar(&options.AuthenticateK8SToken, "authenticate-k8s-token", false, "option to specify if the broker should validate the bearer auth token with kubernetes")
	flag.StringVar(&options.KubeConfig, "kube-config", "", "specify the kube config path to be used")
	flag.BoolVar(&options.Version, "version", false, "output version information and exit")

	// glog to stderr
	flag.Set("logtostderr", "true")

	broker.AddFlags(&options.Options)
	flag.Parse()
}

func main() {
	if options.Version {
		fmt.Println("CloudFront Broker")
		fmt.Printf(" %-30s %s\n", "Version:", Version)
		fmt.Printf(" %-30s %s\n", "Git Commit:", GitCommit)
		fmt.Printf(" %-30s %s\n", "Go Version:", GoVersion)
		fmt.Printf(" %-30s %s\n", "Built:", Built)
		fmt.Printf(" %-30s %s\n", "Open Service Broker Version:", OSBVersion)
	} else if err := run(); err != nil && err != context.Canceled && err != context.DeadlineExceeded {
		glog.Fatalln(err)
	}
}

func run() error {
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	go cancelOnInterrupt(ctx, cancelFunc)

	return runWithContext(ctx)
}

func runWithContext(ctx context.Context) error {
	if flag.Arg(0) == "version" {
		fmt.Printf("%s/%s\n", path.Base(os.Args[0]), "0.1.0")
		return nil
	}
	businessLogic, err := broker.NewBusinessLogic(ctx, options.Options)

	if err != nil {
		return err
	}

	if options.BackgroundTasksOnly {
		glog.V(4).Info("Starting background tasks")
		return businessLogic.RunTasksInBackground(ctx)
		// This should never return
	}

	if (options.TLSCert != "" || options.TLSKey != "") &&
		(options.TLSCert == "" || options.TLSKey == "") {
		fmt.Println("To use TLS with specified cert or key data, both --tlsCert and --tlsKey must be used")
		return nil
	}

	port := strconv.Itoa(options.Port)
	if os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
	} else {
		fmt.Println("port not set, use --port or env var PORT")
		return nil
	}

	addr := ":" + port

	// Prom. metrics
	reg := prom.NewRegistry()
	osbMetrics := metrics.New()
	reg.MustRegister(osbMetrics)

	api, err := rest.NewAPISurface(businessLogic, osbMetrics)
	if err != nil {
		return err
	}

	s := server.New(api, reg)

	businessLogic.AddRoutes(s.Router)

	if options.AuthenticateK8SToken {
		// get k8s client
		k8sClient, err := getKubernetesClient(options.KubeConfig)
		if err != nil {
			return err
		}
		// Create a User Info Authorizer.
		authz := middleware.SARUserInfoAuthorizer{
			SAR: k8sClient.AuthorizationV1().SubjectAccessReviews(),
		}
		// create TokenReviewMiddleware
		tr := middleware.TokenReviewMiddleware{
			TokenReview: k8sClient.AuthenticationV1().TokenReviews(),
			Authorizer:  authz,
		}
		// Use TokenReviewMiddleware.
		s.Router.Use(tr.Middleware)
	}

	glog.Infof("Starting broker!")

	if options.Insecure {
		glog.Warningf("Starting insecure broker")
		err = s.Run(ctx, addr)
	} else {
		if options.TLSCert != "" && options.TLSKey != "" {
			glog.Warningf("Starting secure broker with TLS cert and key data")
			err = s.RunTLS(ctx, addr, options.TLSCert, options.TLSKey)
		} else {
			if options.TLSCertFile == "" || options.TLSKeyFile == "" {
				glog.Error("unable to run securely without TLS Certificate and Key. Please review options and if running with TLS, specify --tls-cert-file and --tls-private-key-file or --tlsCert and --tlsKey.")
				return nil
			}
			glog.Warning("Starting secure broker with file based TLS cert and key")
			err = s.RunTLSWithTLSFiles(ctx, addr, options.TLSCertFile, options.TLSKeyFile)
		}
	}
	return err
}

func getKubernetesClient(kubeConfigPath string) (clientset.Interface, error) {
	var clientConfig *clientrest.Config
	var err error
	if kubeConfigPath == "" {
		clientConfig, err = clientrest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	} else {
		config, err := clientcmd.LoadFromFile(kubeConfigPath)
		if err != nil {
			return nil, err
		}

		clientConfig, err = clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{}).ClientConfig()
		if err != nil {
			return nil, err
		}
	}
	return clientset.NewForConfig(clientConfig)
}

func cancelOnInterrupt(ctx context.Context, f context.CancelFunc) {
	term := make(chan os.Signal)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-term:
			glog.Warning("Received SIGTERM, exiting gracefully...")
			f()
			os.Exit(0)
		case <-ctx.Done():
			os.Exit(0)
		}
	}
}
