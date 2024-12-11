package main

import (
	"embed"
	"strings"
	"time"

	"github.com/pcm720/psu-go"
)

//go:embed nhddl/res/sas/app/*
var iconResources embed.FS // Embeds icon resources

type NHDDLMode string

const (
	NHDDLMode_ALL    = ""
	NHDDLMode_ATA    = "ata"
	NHDDLMode_USB    = "usb"
	NHDDLMode_MX4SIO = "mx4sio"
	NHDDLMode_UDPBD  = "udpbd"
	NHDDLMode_iLink  = "ilink"
)

type NHDDLVMode string

const (
	NHDDLVMode_Default = "default"
	NHDDLVMode_480p    = "480p"
)

type NHDDLConfig struct {
	VMode   string
	UDPBDIP string
	Mode    NHDDLMode
}

// Generates nhddl.yaml
func (c NHDDLConfig) getYAML() string {
	// Not using yaml library saves ~500KB
	b := strings.Builder{}
	if c.VMode == "480p" {
		b.WriteString("480p:\n")
	}
	if c.Mode != NHDDLMode_ALL {
		b.WriteString("mode: " + string(c.Mode) + "\n")
	}
	if c.UDPBDIP != "" {
		b.WriteString("udpbd_ip: " + c.UDPBDIP)
	}

	return b.String()
}

var emptyConfig = NHDDLConfig{
	VMode: NHDDLVMode_Default,
}

func getEmbeddedFiles() ([]psu.File, error) {
	// Get number of embedded files
	entry, err := iconResources.ReadDir("nhddl/res/sas/app")
	if err != nil {
		return nil, err
	}

	files := make([]psu.File, len(entry))
	for i, f := range entry {
		data, err := iconResources.ReadFile("nhddl/res/sas/app/" + f.Name())
		if err != nil {
			return nil, err
		}
		info, err := f.Info()
		if err != nil {
			return nil, err
		}

		files[i] = psu.File{
			Name:     f.Name(),
			Created:  info.ModTime(),
			Modified: info.ModTime(),
			Data:     data,
		}
		if files[i].Created.IsZero() {
			files[i].Created = time.Now()
			files[i].Modified = time.Now()
		}
	}
	return files, nil
}
