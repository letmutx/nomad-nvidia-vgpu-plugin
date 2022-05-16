package device

import (
	"context"
	"fmt"

	"github.com/hashicorp/nomad/plugins/device"
)

// doStats is the long running goroutine that streams device statistics
func (d *NvidiaVgpuDevice) doStats(ctx context.Context, nvStats <-chan *device.StatsResponse, stats chan<- *device.StatsResponse) {
	defer close(stats)

	for {
		select {
		case <-ctx.Done():
			return
		case nvStat := <-nvStats:
			stats <- d.nvStatsToVirtstats(nvStat)
		}
	}
}

func (d *NvidiaVgpuDevice) nvStatsToVirtstats(nvStats *device.StatsResponse) *device.StatsResponse {
	if nvStats.Error != nil {
		return nvStats
	}

	var virtStats device.StatsResponse

	for _, nvStatGroup := range nvStats.Groups {
		group := &device.DeviceGroupStats{
			Name:   nvStatGroup.Name,
			Type:   nvStatGroup.Type,
			Vendor: vendor,
		}

		instanceStats := map[string]*device.DeviceStats{}
		for dev, stats := range group.InstanceStats {
			for i := 0; i < d.vgpus; i++ {
				dev := fmt.Sprintf("%s-%d", dev, i)
				instanceStats[dev] = stats
			}
		}

		group.InstanceStats = instanceStats

		virtStats.Groups = append(virtStats.Groups, group)
	}

	return &virtStats
}
