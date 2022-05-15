package device

import (
	"context"
	"fmt"

	"github.com/hashicorp/nomad/plugins/device"
)

// doFingerprint is the long-running goroutine that detects device changes
func (d *NvidiaVgpuDevice) doFingerprint(ctx context.Context, nvDevices <-chan *device.FingerprintResponse, virtDevices chan *device.FingerprintResponse) {
	defer close(virtDevices)

	for {
		select {
		case <-ctx.Done():
			return
		case nvDevice := <-nvDevices:
			virtDevices <- d.nvDeviceToVirtDevices(ctx, nvDevice)
		}
	}
}

func (d *NvidiaVgpuDevice) nvDeviceToVirtDevices(ctx context.Context, nvFpr *device.FingerprintResponse) *device.FingerprintResponse {
	if nvFpr.Error != nil {
		return nvFpr
	}
	var fpr device.FingerprintResponse

	d.deviceLock.Lock()
	defer d.deviceLock.Unlock()

	for _, nvDeviceGroup := range nvFpr.Devices {
		devGroup := &device.DeviceGroup{
			Name:       nvDeviceGroup.Name,
			Attributes: nvDeviceGroup.Attributes,
			Type:       nvDeviceGroup.Type,
			Vendor:     vendor,
		}

		for _, nvDevice := range nvDeviceGroup.Devices {
			for i := 0; i < d.vgpuMultiplier; i++ {
				dev := &device.Device{
					ID:         fmt.Sprintf("%s-%d", nvDevice.ID, i),
					Healthy:    nvDevice.Healthy,
					HwLocality: nvDevice.HwLocality,
					HealthDesc: nvDevice.HealthDesc,
				}
				d.devices[dev.ID] = struct{}{}
				devGroup.Devices = append(devGroup.Devices, dev)
			}
		}

		fpr.Devices = append(fpr.Devices, devGroup)
	}

	return &fpr
}
