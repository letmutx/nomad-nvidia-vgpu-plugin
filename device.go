package device

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/devices/gpu/nvidia"
	"github.com/hashicorp/nomad/helper/pluginutils/loader"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/device"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
)

const (
	// pluginName is the deviceName of the plugin
	// this is used for logging and (along with the version) for uniquely identifying
	// plugin binaries fingerprinted by the client
	pluginName = "nvidia-vgpu"

	// plugin version allows the client to identify and use newer versions of
	// an installed plugin
	pluginVersion = "v0.1.0"

	// vendor is the label for the vendor providing the devices.
	// along with "type" and "model", this can be used when requesting devices:
	//   https://www.nomadproject.io/docs/job-specification/device.html#name
	vendor = "letmutx"
)

var (
	// pluginInfo provides information used by Nomad to identify the plugin
	pluginInfo = &base.PluginInfoResponse{
		Type:              base.PluginTypeDevice,
		PluginApiVersions: []string{device.ApiVersion010},
		PluginVersion:     pluginVersion,
		Name:              pluginName,
	}

	PluginConfig = &loader.InternalPluginConfig{
		Factory: func(ctx context.Context, l log.Logger) interface{} { return NewPlugin(ctx, l) },
	}

	configSpec = hclspec.NewObject(map[string]*hclspec.Spec{
		"enabled": hclspec.NewDefault(
			hclspec.NewAttr("enabled", "bool", false),
			hclspec.NewLiteral("true"),
		),
		"ignored_gpu_ids": hclspec.NewDefault(
			hclspec.NewAttr("ignored_gpu_ids", "list(string)", false),
			hclspec.NewLiteral("[]"),
		),
		"fingerprint_period": hclspec.NewDefault(
			hclspec.NewAttr("fingerprint_period", "string", false),
			hclspec.NewLiteral("\"1m\""),
		),
		"vgpus": hclspec.NewAttr("vgpus", "number", true),
	})
)

type NvidiaDevicePlugin = device.DevicePlugin

// Config contains configuration information for the plugin.
type Config struct {
	Vgpus int `codec:"vgpus"`
}

// NvidiaVgpuPlugin is a wrapper for NvidiaDevicePlugin
// It handles fingerprinting, stats and allocation of virtual devices
type NvidiaVgpuPlugin struct {
	NvidiaDevicePlugin
	vgpus int

	devices    map[string]struct{}
	deviceLock sync.RWMutex

	log log.Logger
}

// NewPlugin returns a device plugin, used primarily by the main wrapper
//
// Plugin configuration isn't available yet, so there will typically be
// a limit to the initialization that can be performed at this point.
func NewPlugin(ctx context.Context, log log.Logger) *NvidiaVgpuPlugin {
	return &NvidiaVgpuPlugin{
		NvidiaDevicePlugin: nvidia.NewNvidiaDevice(ctx, log),
		devices:            map[string]struct{}{},
		log:                log,
	}
}

// PluginInfo returns information describing the plugin.
//
// This is called during Nomad client startup, while discovering and loading
// plugins.
func (d *NvidiaVgpuPlugin) PluginInfo() (*base.PluginInfoResponse, error) {
	return pluginInfo, nil
}

// ConfigSchema returns the configuration schema for the plugin.
//
// This is called during Nomad client startup, immediately before parsing
// plugin config and calling SetConfig
func (d *NvidiaVgpuPlugin) ConfigSchema() (*hclspec.Spec, error) {
	return configSpec, nil
}

// SetConfig is called by the client to pass the configuration for the plugin.
func (d *NvidiaVgpuPlugin) SetConfig(c *base.Config) (err error) {
	var config Config

	// decode the plugin config
	if err := base.MsgPackDecode(c.PluginConfig, &config); err != nil {
		return err
	}

	if config.Vgpus <= 0 {
		return fmt.Errorf("invalid value for vgpus %q: %w", config.Vgpus, errors.New("must be >= 1"))
	}
	d.vgpus = config.Vgpus

	return d.NvidiaDevicePlugin.SetConfig(c)
}

// Fingerprint streams detected devices.
// Messages should be emitted to the returned channel when there are changes
// to the devices or their health.
func (d *NvidiaVgpuPlugin) Fingerprint(ctx context.Context) (<-chan *device.FingerprintResponse, error) {
	// Fingerprint returns a channel. The recommended way of organizing a plugin
	// is to pass that into a long-running goroutine and return the channel immediately.
	nvOut, err := d.NvidiaDevicePlugin.Fingerprint(ctx)
	if err != nil {
		return nil, err
	}
	outCh := make(chan *device.FingerprintResponse)
	go d.doFingerprint(ctx, nvOut, outCh)
	return outCh, nil
}

// Stats streams statistics for the detected devices.
// Messages should be emitted to the returned channel on the specified interval.
func (d *NvidiaVgpuPlugin) Stats(ctx context.Context, interval time.Duration) (<-chan *device.StatsResponse, error) {
	// Similar to Fingerprint, Stats returns a channel. The recommended way of
	// organizing a plugin is to pass that into a long-running goroutine and
	// return the channel immediately.
	nvOut, err := d.NvidiaDevicePlugin.Stats(ctx, interval)
	if err != nil {
		return nil, err
	}
	outCh := make(chan *device.StatsResponse)
	go d.doStats(ctx, nvOut, outCh)
	return outCh, nil
}

type reservationError struct {
	notExistingIDs []string
}

func (e *reservationError) Error() string {
	return fmt.Sprintf("unknown device IDs: %s", strings.Join(e.notExistingIDs, ","))
}

// Reserve returns information to the task driver on on how to mount the given devices.
// It may also perform any device-specific orchestration necessary to prepare the device
// for use. This is called in a pre-start hook on the client, before starting the workload.
func (d *NvidiaVgpuPlugin) Reserve(deviceIDs []string) (*device.ContainerReservation, error) {
	if len(deviceIDs) == 0 {
		return &device.ContainerReservation{}, nil
	}

	// This pattern can be useful for some drivers to avoid a race condition where a device disappears
	// after being scheduled by the server but before the server gets an update on the fingerprint
	// channel that the device is no longer available.
	d.deviceLock.RLock()
	var notExistingIDs []string
	for _, id := range deviceIDs {
		if _, deviceIDExists := d.devices[id]; !deviceIDExists {
			notExistingIDs = append(notExistingIDs, id)
		}
	}
	d.deviceLock.RUnlock()
	if len(notExistingIDs) != 0 {
		return nil, &reservationError{notExistingIDs}
	}

	nvDevIDs := map[string]struct{}{}
	for _, devID := range deviceIDs {
		idx := strings.LastIndex(devID, "-")
		nvDevIDs[devID[:idx]] = struct{}{}
	}

	devIDs := []string{}
	for nvDevID := range nvDevIDs {
		devIDs = append(devIDs, nvDevID)
	}

	return &device.ContainerReservation{
		Envs: map[string]string{
			nvidia.NvidiaVisibleDevices: strings.Join(devIDs, ","),
		},
	}, nil
}
