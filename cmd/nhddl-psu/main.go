//go:build js && wasm

// WebAssembly UI for NHDDL PSU builder
// Build with TinyGo for significantly smaller executable

package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"syscall/js"
	"time"
	"unsafe"

	"github.com/pcm720/nhddl-psu/gh"
	"github.com/pcm720/psu-go"
)

// Must be set at build time
var (
	Repo      string
	CORSProxy string
)

// Global variables
var (
	ghf *gh.Fetcher
	b   bytes.Buffer // Reusable file buffer
)

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

func displayError(text string) {
	fmt.Println(text)
	js.Global().Call("displayError", text)
}

func getAllTagsWrapper() js.Func {
	jsonFunc := js.FuncOf(func(this js.Value, args []js.Value) any {
		go func() {
			tags, err := ghf.GetAllTags()
			if err != nil {
				displayError(fmt.Sprintf("Failed to get tags: %s", err))
				return
			}
			arr := make([]any, len(tags))
			for i, p := range tags {
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
		b.Reset()

		if len(args) != 3 {
			displayError(fmt.Sprintf("Invalid number of arguments"))
			return nil
		}

		c := NHDDLConfig{
			VMode:   args[0].String(),
			UDPBDIP: args[2].String(),
		}
		for i := 0; i < args[1].Length(); i++ {
			if args[1].Index(i).String() == "auto" {
				continue
			}
			c.Mode = append(c.Mode, NHDDLMode(args[1].Index(i).String()))
		}

		if isConfigEmpty(c) {
			return nil
		}

		b.WriteString(c.getYAML())
		data := b.Bytes()
		js.Global().Call("saveFile", "nhddl.yaml", unsafe.Pointer(&data[0]), len(data))
		return nil
	})
}

func generatePSU() js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		b.Reset()

		if len(args) != 3 {
			displayError(fmt.Sprintf("Invalid number of arguments"))
			return nil
		}

		tag := args[0].String()
		if (tag == "") || (tag == "unknown") {
			return nil
		}

		isStandalone := args[1].Bool()

		c := NHDDLConfig{
			VMode:   args[2].Index(0).String(),
			UDPBDIP: args[2].Index(2).String(),
		}
		for i := 0; i < args[2].Index(1).Length(); i++ {
			if args[2].Index(1).Index(i).String() == "auto" {
				continue
			}
			c.Mode = append(c.Mode, NHDDLMode(args[2].Index(1).Index(i).String()))
		}

		go func(tag string, isStandalone bool, config NHDDLConfig) {
			files, err := getEmbeddedFiles()
			if err != nil {
				displayError(fmt.Sprintf("Failed to get embedded files: %s\n", err))
				return
			}

			if !isConfigEmpty(c) {
				files = append(files, psu.File{
					Name:     "nhddl.yaml",
					Created:  time.Now(),
					Modified: time.Now(),
					Data:     []byte(c.getYAML()),
				})
			}

			targetFile := "nhddl.elf"
			if (tag[0] == 'v') && (tag <= "v1.1.2") {
				// Use standalone version for older releases
				targetFile = "nhddl-standalone.elf"
			}

			elfFile, err := ghf.GetFiles(tag, []string{targetFile})
			if err != nil {
				displayError(fmt.Sprintf("Failed to download ELF: %s\n", err))
				return
			}
			elfFile[0].Name = "nhddl.elf" // Force file name
			files = append(files, elfFile[0])

			if err := psu.BuildPSU(&b, "APP_NHDDL", files); err != nil {
				displayError(fmt.Sprintf("Failed to generate PSU: %s\n", err))
				return
			}
			data := b.Bytes()
			js.Global().Call("saveFile", "nhddl.psu", unsafe.Pointer(&data[0]), len(data))
		}(tag, isStandalone, c)
		return nil
	})
}

func isConfigEmpty(c NHDDLConfig) bool {
	if c.VMode != NHDDLVMode_Default {
		return false
	}
	if len(c.Mode) != 0 {
		return false
	}
	if c.UDPBDIP != "" {
		return false
	}

	return true
}
