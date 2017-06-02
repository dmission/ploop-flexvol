package main

import (
	"os"

	"github.com/jaxxstorm/flexvolume"
	"github.com/kolyshkin/goploop-cli"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "ploop flexvolume"
	app.Usage = "Mount ploop volumes in kubernetes using the flexvolume driver"
	app.Commands = flexvolume.Commands(Ploop{})
	app.CommandNotFound = flexvolume.CommandNotFound
	app.Authors = []cli.Author{
		cli.Author{
			Name: "Lee Briggs",
		},
		cli.Author{
			Name: "Virtuozzo",
		},
	}
	app.Version = "0.2a"
	app.Run(os.Args)
}

type Ploop struct{}

func (p Ploop) Init() flexvolume.Response {
	return flexvolume.Response{
		Status:  flexvolume.StatusSuccess,
		Message: "Ploop is available",
	}
}

func (p Ploop) GetVolumeName(options map[string]string) flexvolume.Response {
	if options["volumePath"] == "" {
		return flexvolume.Response{
			Status:     flexvolume.StatusFailure,
			Message:    "Must specify a volume path",
			VolumeName: "unknown",
		}
	}

	if options["volumeId"] == "" {
		return flexvolume.Response{
			Status:  flexvolume.StatusFailure,
			Message: "Must specify a volume id",
		}
	}

	return flexvolume.Response{
		Status:     flexvolume.StatusSuccess,
		VolumeName: options["volumePath"] + "/" + options["volumeId"],
	}
}

func (p Ploop) Mount(target string, options map[string]string) flexvolume.Response {
	// make the target directory we're going to mount to
	err := os.MkdirAll(target, 0755)
	if err != nil {
		return flexvolume.Response{
			Status:  flexvolume.StatusFailure,
			Message: err.Error(),
		}
	}

	// open the disk descriptor first
	volume, err := ploop.Open(options["volumePath"] + "/" + options["volumeId"] + "/" + "DiskDescriptor.xml")
	if err != nil {
		return flexvolume.Response{
			Status:  flexvolume.StatusFailure,
			Message: err.Error(),
		}
	}
	defer volume.Close()

	if m, _ := volume.IsMounted(); !m {
		// If it's mounted, let's mount it!

		readonly := false
		if options["kubernetes.io/readwrite"] == "ro" {
			readonly = true
		}

		mp := ploop.MountParam{Target: target, Readonly: readonly}

		dev, err := volume.Mount(&mp)
		if err != nil {
			return flexvolume.Response{
				Status:  flexvolume.StatusFailure,
				Message: err.Error(),
				Device:  dev,
			}
		}

		return flexvolume.Response{
			Status:  flexvolume.StatusSuccess,
			Message: "Successfully mounted the ploop volume",
		}
	} else {

		return flexvolume.Response{
			Status:  flexvolume.StatusSuccess,
			Message: "Ploop volume already mounted",
		}

	}
}

func (p Ploop) Unmount(mount string) flexvolume.Response {
	if err := ploop.UmountByMount(mount); err != nil {
		return flexvolume.Response{
			Status:  flexvolume.StatusFailure,
			Message: err.Error(),
		}
	}

	return flexvolume.Response{
		Status:  flexvolume.StatusSuccess,
		Message: "Successfully unmounted the ploop volume",
	}
}
