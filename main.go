package main

import (
	"errors"
	"os"
	"syscall"

	"github.com/jaxxstorm/flexvolume"
	"github.com/kolyshkin/goploop-cli"
	"github.com/urfave/cli"
	"github.com/virtuozzo/ploop-flexvol/vstorage"
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

const WorkingDir = "/var/run/ploop-flexvol/"

func (p Ploop) Init() flexvolume.Response {
	return flexvolume.Response{
		Status:  flexvolume.StatusSuccess,
		Message: "Ploop is available",
	}
}

func (p Ploop) path(options map[string]string) string {
	path := "/"
	if options["volumePath"] != "" {
		path += options["volumePath"] + "/"
	}
	path += options["volumeId"]
	return path
}

func (p Ploop) GetVolumeName(options map[string]string) flexvolume.Response {
	if options["volumeId"] == "" {
		return flexvolume.Response{
			Status:  flexvolume.StatusFailure,
			Message: "Must specify a volume id",
		}
	}

	return flexvolume.Response{
		Status:     flexvolume.StatusSuccess,
		VolumeName: options["volumeId"],
	}
}

func prepareVstorage(options map[string]string, mount string) error {
	mounted, _ := vstorage.IsVstorage(mount)
	if mounted {
		return nil
	}

	if options["kubernetes.io/secret/clusterPassword"] == "" {
		return errors.New("Please provide vstorage credentials")
	}

	// not mounted in proper place, prepare mount place and check other
	// mounts
	if err := os.MkdirAll(mount, 0755); err != nil {
		return err
	}

	v := vstorage.Vstorage{options["kubernetes.io/secret/clusterName"]}
	p, _ := v.Mountpoint()
	if p != "" {
		return syscall.Mount(p, mount, "", syscall.MS_BIND, "")
	}

	if err := v.Auth(options["kubernetes.io/secret/clusterPassword"]); err != nil {
		return err
	}
	if err := v.Mount(mount); err != nil {
		return err
	}

	return nil
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

	path := p.path(options)

	if options["kubernetes.io/secret/clusterName"] != "" {
		mount := WorkingDir + options["kubernetes.io/secret/clusterName"]
		if err := prepareVstorage(options, mount); err != nil {
			return flexvolume.Response{
				Status:  flexvolume.StatusFailure,
				Message: err.Error(),
			}
		}
		path = mount + path
	}
	// open the disk descriptor first
	volume, err := ploop.Open(path + "/" + "DiskDescriptor.xml")
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
