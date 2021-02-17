module cloudfront-broker

go 1.12

require (
	github.com/Masterminds/semver v1.5.0
	github.com/aws/aws-sdk-go v1.37.12
	github.com/fatih/structs v1.1.0
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/gorilla/mux v1.7.3
	github.com/kubernetes/client-go v11.0.0+incompatible // indirect
	github.com/lib/pq v1.2.0
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d
	github.com/pkg/errors v0.9.1
	github.com/pmorie/go-open-service-broker-client v0.0.0-20180912182616-9cc214e88d00
	github.com/pmorie/osb-broker-lib v0.0.0-20180423023500-052cd99aa13d
	github.com/prometheus/client_golang v0.9.4
	github.com/shawn-hurley/osb-broker-k8s-lib v0.0.0-20180430125558-bed19ac36ffe
	github.com/smartystreets/goconvey v0.0.0-20190330032615-68dc04aab96a
	github.com/spf13/pflag v1.0.3 // indirect
	k8s.io/client-go v0.0.0-20190602130007-e65ca70987a6
)
