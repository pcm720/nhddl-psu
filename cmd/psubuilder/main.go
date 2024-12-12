//go:build !js

// Small PSU builder utility that can build PSU from local files or from remote GitHub repository
package main

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"github.com/pcm720/nhddl-psu/gh"
	"github.com/pcm720/psu-go"
	"github.com/urfave/cli"
)

var Version = ""

func main() {
	app := &cli.App{
		Name:        "psubuilder",
		Description: "Builds PSU from local files or GitHub releases",
		Version:     Version,
		Commands: []cli.Command{
			{
				Name:  "tags",
				Usage: "Get release tags",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:     "repo",
						Usage:    "GitHub repository to get releases from",
						EnvVar:   "TARGET_REPO",
						Required: true,
					},
				},
				Action: func(ctx *cli.Context) error {
					ghf := &gh.Fetcher{
						Repo: ctx.String("repo"),
					}

					tags, err := ghf.GetAllTags()
					if err != nil {
						return err
					}

					fmt.Println("Available tags:")
					for _, t := range tags {
						fmt.Println(t)
					}
					return nil
				},
			},
			{
				Name:  "psu",
				Usage: "Build PSU. Accepts output file name in the first argument, uses out.psu as default",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:     "dirname",
						Usage:    "PSU directory name",
						EnvVar:   "PSU_DIR",
						Required: true,
					},
					cli.StringFlag{
						Name:   "tag",
						Usage:  "Release tag",
						EnvVar: "RELEASE_TAG",
						Value:  "nightly",
					},
					cli.StringSliceFlag{
						Name:     "file",
						Usage:    "File or directory to include. Multiple files can be specified by repeating this flag. In env variable, multiple files are separated by comma. Files in ZIP release require full path (e.g. dir1/dir2/file).",
						EnvVar:   "TARGET_FILES",
						Required: true,
					},
					cli.StringFlag{
						Name:   "repo",
						Usage:  "GitHub repository to get releases from. If not set, 'files' will be treated as local paths",
						EnvVar: "TARGET_REPO",
					},
				},
				Action: func(ctx *cli.Context) error {
					var files []psu.File
					if ctx.String("repo") == "" {
						lfiles, err := getLocalFiles(ctx.StringSlice("file"))
						if err != nil {
							return err
						}
						files = append(files, lfiles...)
					} else {
						ghf := &gh.Fetcher{
							Repo: ctx.String("repo"),
						}
						zipFiles, err := ghf.GetFiles(ctx.String("tag"), ctx.StringSlice("file"))
						if err != nil {
							return err
						}
						files = append(files, zipFiles...)
					}

					targetFilename := "out.psu"
					if ctx.Args().Get(0) != "" {
						targetFilename = ctx.Args().Get(0)
					}
					w, err := os.OpenFile(targetFilename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0664)
					if err != nil {
						return err
					}
					defer w.Close()
					if err := psu.BuildPSU(w, ctx.String("dirname"), files); err != nil {
						return err
					}
					fmt.Println("PSU built successfully")
					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Println(err)
	}
}

// Parses filenames into psu.Files
// Handles directories recursively
func getLocalFiles(filenames []string) ([]psu.File, error) {
	var res []psu.File
	for _, f := range filenames {
		fmt.Printf("processing %s\n", f)
		files, err := processFile(f)
		if err != nil {
			return nil, err
		}
		res = append(res, files...)
	}
	return res, nil
}

// Reads files and directories recursively
func processFile(name string) ([]psu.File, error) {
	var res []psu.File
	lf, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer lf.Close()

	info, err := lf.Stat()
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		entries, err := lf.ReadDir(0)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			fullPath := path.Join(name, e.Name())
			fmt.Printf("processing %s\n", fullPath)
			files, err := processFile(fullPath)
			if err != nil {
				return nil, err
			}
			res = append(res, files...)
		}
		return res, nil
	}

	data, err := io.ReadAll(lf)
	if err != nil {
		return nil, err
	}

	mTime := info.ModTime()
	if mTime.IsZero() {
		mTime = time.Now()
	}

	res = append(res, psu.File{
		Name:     path.Base(lf.Name()),
		Created:  mTime,
		Modified: mTime,
		Data:     data,
	})

	return res, nil
}
