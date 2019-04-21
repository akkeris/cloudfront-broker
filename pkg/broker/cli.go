package broker

import (
  "flag"
)

// Options holds the options specified by the broker's code on the command
// line. Users should add their own options here and add flags for them in
// AddFlags.
type Options struct {
  CatalogPath         string
  Async               bool
  DatabaseUrl         string
  NamePrefix          string
  WaitSecs            int64
  MaxRetries					int64
  BackgroundTasksOnly bool
}

// AddFlags is a hook called to initialize the CLI flags for broker options.
// It is called after the flags are added for the skeleton and before flag
// parse is called.
func AddFlags(o *Options) {
  flag.StringVar(&o.CatalogPath, "catalogPath", "", "The path to the catalog")
  flag.BoolVar(&o.Async, "async", false, "Indicates whether the broker is handling the requests asynchronously.")
  flag.StringVar(&o.DatabaseUrl, "database-url", "", "The database url to use for storage (e.g., postgres://user:pass@host:port/dbname), you can also set DATABASE_URL environment var.")
  flag.StringVar(&o.NamePrefix, "name-prefix", "", "Prefix for S3 bucket name, can also be set with NAME_PREFIX environment var.")
  flag.Int64Var(&o.WaitSecs, "wait-seconds", 15, "Seconds to wait between aws operations checks, can also be set with WAIT_SECONDS environment var.")
  flag.Int64Var(&o.MaxRetries, "max-retries", 100, "Number of checks for a service to complete before giving an error")
  flag.BoolVar(&o.BackgroundTasksOnly, "backgroundtasksonly", false, "run background tasks only")
}
