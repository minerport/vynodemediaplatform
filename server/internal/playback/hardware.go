package playback

import (
	"os"
	"runtime"
)

type HardwareBackend struct {
	Type      string `json:"type"`
	Detected  bool   `json:"detected"`
	Available bool   `json:"available"`
	Decode    bool   `json:"decode"`
	Encode    bool   `json:"encode"`
	Encoder   string `json:"encoder,omitempty"`
	Status    string `json:"status"`
}

func detectHardware(c FFmpegCapabilities) []HardwareBackend {
	backends := []HardwareBackend{{Type: "SOFTWARE", Detected: contains(c.Encoders, "libx264"), Available: contains(c.Encoders, "libx264"), Decode: true, Encode: contains(c.Encoders, "libx264"), Encoder: "libx264", Status: "FFMPEG_ENCODER_AVAILABLE"}}
	types := []struct{ name, encoder, device string }{{"NVIDIA", "h264_nvenc", "/dev/nvidia0"}, {"INTEL_QSV", "h264_qsv", "/dev/dri/renderD128"}, {"VAAPI", "h264_vaapi", "/dev/dri/renderD128"}, {"AMD_AMF", "h264_amf", ""}, {"VIDEOTOOLBOX", "h264_videotoolbox", ""}}
	for _, x := range types {
		present := contains(c.Encoders, x.encoder)
		device := false
		if x.device != "" {
			_, e := os.Stat(x.device)
			device = e == nil
		} else if x.name == "AMD_AMF" {
			device = runtime.GOOS == "windows"
		} else if x.name == "VIDEOTOOLBOX" {
			device = runtime.GOOS == "darwin"
		}
		available := present && device
		status := "NOT_DETECTED"
		if present && !device {
			status = "ENCODER_PRESENT_DEVICE_UNAVAILABLE"
		}
		if available {
			status = "DETECTED_NOT_RUNTIME_VALIDATED"
		}
		backends = append(backends, HardwareBackend{Type: x.name, Detected: present && device, Available: false, Decode: false, Encode: present, Encoder: x.encoder, Status: status})
	}
	return backends
}
