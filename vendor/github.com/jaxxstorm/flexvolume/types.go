package flexvolume

type Status string

const (
	StatusSuccess      Status = "Success"
	StatusFailure      Status = "Failure"
	StatusNotSupported Status = "Not supported"
)

type FlexVolume interface {
	Init() Response
	Mount(string, map[string]string) Response
	Unmount(string) Response
	GetVolumeName(map[string]string) Response
}

type Response struct {
	Status     Status `json:"status"`
	Message    string `json:"message,omitempty"`
	Device     string `json:"device,omitempty"`
	VolumeName string `json:"volumeName,omitempty"`
}
