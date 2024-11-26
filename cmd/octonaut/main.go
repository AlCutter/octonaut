package main

import (
	"flag"

	"github.com/AlCutter/octonaut/cmd/octonaut/cmd"
	"github.com/spf13/pflag"
	"k8s.io/klog/v2"
)

func main() {
	klog.InitFlags(nil)
	// Make cobra aware of select glog flags
	// Enabling all flags causes unwanted deprecation warnings
	// from glog to always print in plugin mode
	pflag.CommandLine.AddGoFlag(flag.CommandLine.Lookup("v"))
	pflag.CommandLine.AddGoFlag(flag.CommandLine.Lookup("logtostderr"))
	pflag.CommandLine.Set("logtostderr", "true")

	cmd.Execute()
}
