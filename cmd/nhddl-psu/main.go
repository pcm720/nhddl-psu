//go:build js && wasm

// WebAssembly UI for NHDDL PSU builder
// Build with TinyGo for significantly smaller executable

package main

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"fmt"
	"syscall/js"
	"time"

	"github.com/pcm720/nhddl-psu/gh"
	"github.com/pcm720/psu-go"
)

// Must be set at build time
var Repo string
var CORSProxy string

var ghf *gh.Fetcher

func main() {
	if Repo == "" {
		fmt.Println("repository not set")
		return
	}
	fmt.Printf("using repository %s\nCORS proxy is '%s'\n", Repo, CORSProxy)
	ghf = &gh.Fetcher{
		Repo:      Repo,
		CORSProxy: CORSProxy,
	}
	js.Global().Set("getAllTags", getAllTagsWrapper())
	js.Global().Call("updateTags")
	js.Global().Set("buildPSU", generatePSU())
	js.Global().Set("getNHDDLConfig", getNHDDLConfig())
	<-make(chan struct{})
}

func getAllTagsWrapper() js.Func {
	jsonFunc := js.FuncOf(func(this js.Value, args []js.Value) any {
		go func() {
			pretty, err := ghf.GetAllTags()
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

		return c.getYAML()
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
				files = append(files, psu.File{
					Name:     "nhddl.yaml",
					Created:  time.Now(),
					Modified: time.Now(),
					Data:     []byte(c.getYAML()),
				})
			}

			targetFile := "nhddl.elf"
			if isStandalone {
				targetFile = "nhddl-standalone.elf"
			}

			elfFile, err := ghf.GetFiles(tag, []string{targetFile})
			if err != nil {
				fmt.Printf("failed to download ELF: %s\n", err)
				return
			}
			elfFile[0].Name = "nhddl.elf" // Force file name
			files = append(files, elfFile[0])

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
