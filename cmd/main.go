package main

import (
	"context"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/plugins"
	device "github.com/letmutx/nomad-nvidia-vgpu-plugin"
)

func main() {
	// Serve the plugin
	plugins.Serve(factory)
}

// factory returns a new instance of our example device plugin
func factory(log log.Logger) interface{} {
	return device.NewPlugin(context.Background(), log)
}
