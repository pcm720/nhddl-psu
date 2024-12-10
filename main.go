//go:build js && wasm

package main

import (
	"bytes"
	"embed"
	_ "embed"
	"encoding/base64"
	"fmt"
	"syscall/js"
	"time"

	"github.com/pcm720/psu-go"
	"gopkg.in/yaml.v3"
)

//go:embed nhddl/res/sas/app/*
var iconResources embed.FS

func getEmbeddedFiles() ([]psu.File, error) {
	// Get number of embedded files
	entry, err := iconResources.ReadDir("app")
	if err != nil {
		return nil, err
	}

	files := make([]psu.File, len(entry))
	for i, f := range entry {
		data, err := iconResources.ReadFile("app/" + f.Name())
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
	}
	return files, nil
}

type NHDDLMode string

const (
	NHDDLMode_ALL    = "all"
	NHDDLMode_ATA    = "ata"
	NHDDLMode_USB    = "usb"
	NHDDLMode_MX4SIO = "mx4sio"
	NHDDLMode_UDPBD  = "udpbd"
	NHDDLMode_iLink  = "ilink"
)

type NHDDLConfig struct {
	Use480p bool      `yaml:"480p,omitempty"`
	UDPBDIP string    `yaml:"udpbd_ip,omitempty"`
	Mode    NHDDLMode `yaml:"mode,omitempty"`
}

var emptyConfig = NHDDLConfig{}

func getAllTagsWrapper() js.Func {
	jsonFunc := js.FuncOf(func(this js.Value, args []js.Value) any {
		go func() {
			pretty, err := getAllTags()
			if err != nil {
				fmt.Printf("unable to convert to json %s\n", err)
				return
			}
			arr := make([]any, len(pretty))
			for i, p := range pretty {
				arr[i] = p
			}
			js.Global().Call("setTagList", arr)
		}()
		return nil
	})
	return jsonFunc
}

func getNHDDLConfig() js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) != 3 {
			fmt.Println("invalid number of arguments")
			return nil
		}

		c := NHDDLConfig{
			Use480p: args[0].Bool(),
			UDPBDIP: args[2].String(),
			Mode:    NHDDLMode(args[1].String()),
		}

		if c == emptyConfig {
			return nil
		}
		res, err := yaml.Marshal(c)
		if err != nil {
			fmt.Printf("failed to generate YAML: %s\n", err)
			return nil
		}

		return string(res)
	})
}

func generatePSU() js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) != 3 {
			fmt.Println("invalid number of arguments")
			return nil
		}

		tag := args[0].String()
		if tag == "" {
			return nil
		}

		isStandalone := args[1].Bool()

		c := NHDDLConfig{
			Use480p: args[2].Index(0).Bool(),
			UDPBDIP: args[2].Index(2).String(),
			Mode:    NHDDLMode(args[2].Index(1).String()),
		}

		go func(tag string, isStandalone bool, config NHDDLConfig) {
			files, err := getEmbeddedFiles()
			if err != nil {
				fmt.Printf("failed to get embedded files: %s\n", err)
				return
			}

			if c != emptyConfig {
				res, err := yaml.Marshal(c)
				if err != nil {
					fmt.Printf("failed to generate YAML: %s\n", err)
					return
				}

				files = append(files, psu.File{
					Name:     "nhddl.yaml",
					Created:  time.Now(),
					Modified: time.Now(),
					Data:     res,
				})
			}

			elfFile, err := downloadELF(tag, isStandalone)
			if err != nil {
				fmt.Printf("failed to download ELF: %s\n", err)
				return
			}
			files = append(files, elfFile)

			b := &bytes.Buffer{}
			if err := psu.BuildPSU(b, "NHDDL", files); err != nil {
				fmt.Printf("failed to generate PSU: %s\n", err)
				return
			}
			js.Global().Call("downloadFile", "nhddl.psu", base64.RawStdEncoding.EncodeToString(b.Bytes()))
		}(tag, isStandalone, c)
		return nil
	})
}

func main() {
	js.Global().Set("getAllTags", getAllTagsWrapper())
	js.Global().Call("updateTags")
	js.Global().Set("buildPSU", generatePSU())
	js.Global().Set("getNHDDLConfig", getNHDDLConfig())
	<-make(chan struct{})
}
