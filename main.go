package main

import (
	"crypto/md5"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/jaxxstorm/flexvolume"
	"github.com/kolyshkin/goploop-cli"
	"github.com/urfave/cli"
	"github.com/virtuozzo/ploop-flexvol/vstorage"

	"github.com/golang/glog"
)

func setupJournld() ([]string, *exec.Cmd, error) {
	fd, err := syscall.Dup(syscall.Stdout)
	if err != nil {
		return nil, nil, err
	}

	syscall.CloseOnExec(fd)

	flexvolume.SetRespFile(os.NewFile((uintptr)(fd), "RespFile"))

	if err := flag.CommandLine.Parse([]string{"-logtostderr"}); err != nil {
		return nil, nil, err
	}

	cmd := exec.Command("systemd-cat", "--identifier", "ploop-flexvol")
	if err != nil {
		return nil, nil, err
	}
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, nil, fmt.Errorf("Unable to create a pipe: %v", err)
	}
	cmd.Stdin = pr
	defer pr.Close()
	defer pw.Close()

	if err := syscall.Dup2(int(pw.Fd()), syscall.Stdout); err != nil {
		return nil, nil, fmt.Errorf("Unable to redirect stdout: %v", err)
	}
	if err := syscall.Dup2(syscall.Stdout, syscall.Stderr); err != nil {
		return nil, nil, fmt.Errorf("Unable to redirect stderr: %v", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("Unable to start systemd-cat: %v", err)
	}
	return os.Args, cmd, nil
}

func setupWrapperLogging() ([]string, *exec.Cmd, error) {
	syscall.CloseOnExec(3)
	flexvolume.SetRespFile(os.NewFile((uintptr)(3), "RespFile"))
	if err := flag.CommandLine.Parse(os.Args[2:]); err != nil {
		return nil, nil, err
	}
	return flag.CommandLine.Args(), nil, nil
}

func setupLogging() ([]string, *exec.Cmd, error) {
	if os.Args[1] == "wrapper" {
		return setupWrapperLogging()
	}

	return setupJournld()
}

func main() {
	args, cmd, err := setupLogging()
	if err != nil {
		panic(err)
	}
	if cmd != nil {
		defer func() {
			syscall.Close(syscall.Stdout)
			syscall.Close(syscall.Stderr)
			cmd.Wait()
		}()
	}

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

	if glog.V(4) {
		glog.Infof("Request: %v", args)
	}
	app.Run(args)
}

type Ploop struct{}

const workingDir = "/var/run/ploop-flexvol/"

func (p Ploop) Init() (*flexvolume.Response, error) {
	return &flexvolume.Response{
		Status:  flexvolume.StatusSuccess,
		Message: "Ploop is available",
	}, nil
}

func (p Ploop) path(options map[string]string) string {
	path := "/"
	if options["volumePath"] != "" {
		path += options["volumePath"] + "/"
	}
	path += options["volumeID"]
	return path
}

func (p Ploop) GetVolumeName(options map[string]string) (*flexvolume.Response, error) {
	return &flexvolume.Response{
		Status: flexvolume.StatusNotSupported,
	}, nil
}

func prepareVstorage(clusterName, clusterPasswd string, mount string) error {
	mounted, _ := vstorage.IsVstorage(mount)
	if mounted {
		return nil
	}

	// not mounted in proper place, prepare mount place and check other
	// mounts
	if err := os.MkdirAll(mount, 0700); err != nil {
		return err
	}

	v := vstorage.Vstorage{clusterName}
	p, _ := v.Mountpoint()
	if p != "" {
		return syscall.Mount(p, mount, "", syscall.MS_BIND, "")
	}

	if clusterPasswd == "" {
		return errors.New("Please provide vstorage credentials")
	}

	if err := v.Auth(clusterPasswd); err != nil {
		return err
	}
	if err := v.Mount(mount); err != nil {
		return err
	}

	return nil
}

func (p Ploop) mountPloop(target, path string, volume *ploop.Ploop, readonly bool) (string, error) {
	target = filepath.Clean(target)
	path = filepath.Clean(path)

	statePath := fmt.Sprintf("%s/mounts/ploop-%x", workingDir, md5.Sum([]byte(path)))
	mntPath := fmt.Sprintf("%s/mnt", statePath)

	if err := os.MkdirAll(mntPath, 0700); err != nil {
		return "", err
	}
	mp := ploop.MountParam{Target: mntPath, Readonly: readonly}

	_, err := volume.Mount(&mp)
	if err != nil {
		os.Remove(mntPath)
		os.Remove(statePath)
		return "", err
	}

	return statePath, nil
}

func (p Ploop) umountPloop(statePath string) error {
	mountPath := fmt.Sprintf("%s/mnt", statePath)
	if err := ploop.UmountByMount(mountPath); err != nil {
		return err
	}

	if err := os.Remove(mountPath); err != nil {
		return fmt.Errorf("Unable to remove %s: %v", mountPath, err)
	}

	if err := os.Remove(statePath); err != nil {
		return fmt.Errorf("Unable to remove %s: %v", statePath, err)
	}

	return nil
}

func (p Ploop) Mount(target string, options map[string]string) (*flexvolume.Response, error) {
	path := p.path(options)

	readonly := false
	if options["kubernetes.io/readwrite"] == "ro" {
		readonly = true
	}

	if options["kubernetes.io/secret/clusterName"] != "" {
		_cluster, err := base64.StdEncoding.DecodeString(options["kubernetes.io/secret/clusterName"])
		if err != nil {
			return nil, fmt.Errorf("Unable to decode a cluster name: %v", err.Error())
		}
		cluster := string(_cluster)

		_passwd, err := base64.StdEncoding.DecodeString(options["kubernetes.io/secret/clusterPassword"])
		if err != nil {
			return nil, fmt.Errorf("Unable to decode a cluster password: %v", err.Error())
		}
		passwd := string(_passwd)

		mount := workingDir + cluster
		if err := prepareVstorage(cluster, passwd, mount); err != nil {
			return nil, err
		}
		path = mount + path

		if !readonly {
			// Node denial may lead to vstorage freezes. vstorage revoke operation before writing
			// data will prevent this cases. Detach method is more suitable for it, but currently
			// volume name is auto generated and does not include all neccessary credentials to
			// perform volume revoke. It should be fixed when k8s community fixed getvolumename call
			v := vstorage.Vstorage{cluster}
			if err := v.Revoke(path); err != nil {
				return nil, err
			}
		}
	}
	// open the disk descriptor first
	volume, err := ploop.Open(path + "/" + "DiskDescriptor.xml")
	if err != nil {
		return nil, err
	}
	defer volume.Close()

	if m, _ := volume.IsMounted(); !m {
		stateDir := fmt.Sprintf("%s/mounts", workingDir)
		if err := os.MkdirAll(stateDir, 0700); err != nil {
			return nil, err
		}

		statePath, err := p.mountPloop(target, path, &volume, readonly)
		if err != nil {
			return nil, err
		}

		target = filepath.Clean(target)

		// We need to know a mount point to make snapshots, so
		// we create our mount point and then bind-mount it to "target"
		// If it's mounted, let's mount it!
		mntLink := fmt.Sprintf("%s/kube-%x", stateDir, md5.Sum([]byte(target)))

		glog.Infof("Create symlink %s %s", statePath, mntLink)
		if err := os.Symlink(statePath, mntLink); err != nil {
			p.umountPloop(statePath)
			return nil, err
		}

		mntPath := fmt.Sprintf("%s/mnt", statePath)
		if err := syscall.Mount(mntPath, target, "", syscall.MS_BIND, ""); err != nil {
			p.umountPloop(statePath)
			os.Remove(mntLink)
			return nil, fmt.Errorf("Unable to bind mount %s -> %s: %v", mntPath, target, err)
		}

		return &flexvolume.Response{
			Status:  flexvolume.StatusSuccess,
			Message: "Successfully mounted the ploop volume",
		}, nil
	} else {

		return nil, fmt.Errorf("Ploop volume already mounted")
	}
}

func (p Ploop) Unmount(mount string) (*flexvolume.Response, error) {
	if err := syscall.Unmount(mount, 0); err != nil {
		return nil, err
	}

	mount = filepath.Clean(mount)

	mntLink := fmt.Sprintf("%s/mounts/kube-%x", workingDir, md5.Sum([]byte(mount)))
	statePath, err := os.Readlink(mntLink)
	if err != nil {
		return nil, err
	}

	glog.Infof("Umount %s(%s)", statePath, mntLink)
	if err := p.umountPloop(statePath); err != nil {
		return nil, err
	}

	if err := os.Remove(mntLink); err != nil {
		return nil, err
	}

	return &flexvolume.Response{
		Status:  flexvolume.StatusSuccess,
		Message: "Successfully unmounted the ploop volume",
	}, nil
}

func (p Ploop) Attach(nodename string, options map[string]string) (*flexvolume.Response, error) {
	return &flexvolume.Response{
		Status:  flexvolume.StatusSuccess,
		Message: fmt.Sprintf("Successfully attached the ploop volume to node %s", nodename),
	}, nil
}

func (p Ploop) Detach(device string, nodename string) (*flexvolume.Response, error) {
	return &flexvolume.Response{
		Status:  flexvolume.StatusSuccess,
		Message: fmt.Sprintf("Successfully detached the ploop volume %s from node %s", device, nodename),
	}, nil
}
