Nomad Nvidia Virtual Device Plugin
==================

This repo contains a device plugin for [Nomad](https://www.nomadproject.io/) to support exposing a number of virtual GPUs for each physical GPU present on the machine. This enables running workloads which don't consume the whole GPU.

Installation requirements
-----------------------

This plugin needs the following dependencies to function:

* [Nomad](https://www.nomadproject.io/downloads.html) 0.9+
* GNU/Linux x86_64 with kernel version > 3.10
* NVIDIA GPU with Architecture > Fermi (2.1)
* NVIDIA drivers >= 340.29 with binary nvidia-smi
* Docker v19.03+
 
Copy the plugin binary to the [plugins directory](https://www.nomadproject.io/docs/configuration/index.html#plugin_dir) and [configure the plugin](https://www.nomadproject.io/docs/configuration/plugin.html) in the client config. Also, see the requirements for the official [nvidia-plugin](https://www.nomadproject.io/plugins/devices/nvidia#installation-requirements). 

```hcl
plugin "nvidia-vgpu" {
  config {
    ignored_gpu_ids    = ["uuid1", "uuid2"]
    fingerprint_period = "5s"
    vgpus = 16
  }
}
```

Usage
--------------

Then use the [device stanza](https://www.nomadproject.io/docs/job-specification/device.html) in the job file to schedule with device support.

```hcl
job "gpu-test" {
  datacenters = ["dc1"]
  type = "batch"

  group "smi" {
    task "smi" {
      driver = "docker"

      config {
        image = "nvidia/cuda:11.0-base"
        command = "nvidia-smi"
      }

      resources {
        device "nvidia-vgpu/gpu" {
          count = 1

          # Add an affinity for a particular model
          affinity {
            attribute = "${device.model}"
            value     = "Tesla K80"
            weight    = 50
          }
        }
      }
    }
  }
}
```


