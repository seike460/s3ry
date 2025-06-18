// Package main implements the Terraform Provider for S3ry
// Provides infrastructure-as-code management for S3ry operations and configurations
package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/seike460/terraform-provider-s3ry/internal/provider"
)

// version will be set by the goreleaser configuration
// to appropriate Git tag / commit hash during build
var version = "dev"

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/seike460/s3ry",
		Debug:   debug,
	}

	err := providerserver.Serve(context.Background(), provider.New(version), opts)
	if err != nil {
		log.Fatal(err.Error())
	}
}
