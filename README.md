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

Use the [device stanza](https://www.nomadproject.io/docs/job-specification/device.html) in the job file to schedule with device support.

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
        device "letmutx/gpu" {
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

Notes
-------

* GPU memory allocation/usage is handled in a cooperative manner. This means that one bad GPU process using more memory than assigned can cause starvation for other processes.
* Managing memory isolation per task is left to the user. It depends on a lot of factors like [MPS](https://docs.nvidia.com/deploy/mps/index.html#topic_3_3_3), GPU architecture etc. [This doc](https://drops.dagstuhl.de/opus/volltexte/2018/8984/pdf/LIPIcs-ECRTS-2018-20.pdf) has some information.

Testing
---------
The best way to test the plugin is to go to a target machine with Nvidia GPU and run the plugin using Nomad's [plugin launcher](https://github.com/hashicorp/nomad/blob/main/plugins/shared/cmd/launcher/README.md) with:

```shell
make eval
```

Inspired by
--------------

* https://github.com/awslabs/aws-virtual-gpu-device-plugin
* https://github.com/kubernetes/kubernetes/issues/52757#issuecomment-402772200
