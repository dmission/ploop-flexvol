package flexvolume

type Status string

const (
	StatusSuccess      Status = "Success"
	StatusFailure      Status = "Failure"
	StatusNotSupported Status = "Not supported"
)

type FlexVolume interface {
	Init() (*Response, error)
	Mount(string, map[string]string) (*Response, error)
	Unmount(string) (*Response, error)
	Attach(string, map[string]string) (*Response, error)
	Detach(string, string) (*Response, error)
	GetVolumeName(map[string]string) (*Response, error)
}

type Response struct {
	Status     Status `json:"status"`
	Message    string `json:"message,omitempty"`
	Device     string `json:"device,omitempty"`
	VolumeName string `json:"volumeName,omitempty"`
}
