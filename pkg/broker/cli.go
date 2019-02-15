package broker

import (
	"flag"
)

// Options holds the options specified by the broker's code on the command
// line. Users should add their own options here and add flags for them in
// AddFlags.
type Options struct {
	CatalogPath string
	Async       bool
	DatabaseUrl string
	NamePrefix  string
}

// AddFlags is a hook called to initialize the CLI flags for broker options.
// It is called after the flags are added for the skeleton and before flag
// parse is called.
func AddFlags(o *Options) {
	flag.StringVar(&o.CatalogPath, "catalogPath", "", "The path to the catalog")
	flag.BoolVar(&o.Async, "async", false, "Indicates whether the broker is handling the requests asynchronously.")
	flag.StringVar(&o.DatabaseUrl, "database-url", "", "The database url to use for storage (e.g., postgres://user:pass@host:port/dbname), you can also set DATABASE_URL environment var.")
	flag.StringVar(&o.NamePrefix, "NamePrefix", "", "Prefix for S3 bucket name, can also be set with NAME_PREFIX environment var.")
}
