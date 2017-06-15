package flexvolume

import (
	"encoding/json"
	"fmt"
	"github.com/urfave/cli"
)

func CommandNotFound(c *cli.Context, command string) {
	handle(&Response{
		Status: StatusNotSupported,
	}, nil)
}

func Commands(fv FlexVolume) []cli.Command {
	return []cli.Command{
		{
			Name:  "init",
			Usage: "Initialize the driver",
			Action: func(c *cli.Context) error {
				return handle(fv.Init())
			},
		},
		{
			Name:  "getvolumename",
			Usage: "Get a cluster wide unique volume name for the volume",
			Action: func(c *cli.Context) error {
				var opts map[string]string
				if err := json.Unmarshal([]byte(c.Args().Get(0)), &opts); err != nil {
					return err
				}
				return handle(fv.GetVolumeName(opts))
			},
		},
		{
			Name:  "unmount",
			Usage: "Detach the volume",
			Action: func(c *cli.Context) error {
				return handle(fv.Unmount(c.Args().Get(0)))
			},
		},
		{
			Name:  "mount",
			Usage: "Mount the volume",
			Action: func(c *cli.Context) error {
				var opts map[string]string

				if err := json.Unmarshal([]byte(c.Args().Get(1)), &opts); err != nil {
					return err
				}

				return handle(fv.Mount(c.Args().Get(0), opts))
			},
		},
	}
}

// The following handles:
//   * Output of the Response object.
//   * Sets an error so we can bubble up an error code.
func handle(resp *Response, err error) error {
	if err != nil {
		resp = &Response{
			Status: StatusFailure,
			Message: err.Error(),
		}	
	}

	// Format the output as JSON.
	output, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	fmt.Println(string(output))
	return nil
}
