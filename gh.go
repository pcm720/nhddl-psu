package main

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/go-github/v67/github"
	"github.com/pcm720/psu-go"
)

var ghClient = github.NewClient(nil)

func getReleaseURL(tag string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	release, _, err := ghClient.Repositories.GetReleaseByTag(ctx, "pcm720", "nhddl", tag)
	if err != nil {
		return "", err
	}
	if len(release.Assets) < 1 {
		return "", errors.New("no assets")
	}

	return *release.Assets[0].BrowserDownloadURL, nil
}

func getAllTags() ([]string, error) {
	tags, _, err := ghClient.Repositories.ListTags(context.Background(), "pcm720", "nhddl", nil)
	if err != nil {
		return nil, err
	}

	tagNames := make([]string, len(tags))
	for i, tag := range tags {
		if tag.GetName() == "nightly" {
			// Make sure nightly tag is always first
			t := tagNames[0]
			tagNames[0] = tag.GetName()
			for j := 1; j <= i; j++ {
				tagNames[j], t = t, tagNames[j]
			}
			continue
		}
		tagNames[i] = tag.GetName()
	}

	return tagNames, nil
}

func downloadELF(tag string, isStandalone bool) (psu.File, error) {
	fmt.Println("getting release ZIP for", tag)
	rel, err := getReleaseURL(tag)
	if err != nil {
		return psu.File{}, err
	}

	fmt.Println("downloading", rel)
	httpClient := &http.Client{
		Timeout: 5 * time.Second,
	}
	req, err := http.NewRequest("GET", rel, nil)
	if err != nil {
		return psu.File{}, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return psu.File{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return psu.File{}, fmt.Errorf("invalid status code %d", resp.StatusCode)
	}

	zipData, err := io.ReadAll(resp.Body)
	if err != nil {
		return psu.File{}, err
	}

	fmt.Println("size", resp.ContentLength)

	fmt.Println("opening file", rel)
	z, err := zip.NewReader(bytes.NewReader(zipData), resp.ContentLength)
	if err != nil {
		return psu.File{}, err
	}

	fmt.Println("processing ZIP archive")
	targetName := regularELF
	if isStandalone {
		targetName = standaloneELF
	}
	for _, f := range z.File {
		if f.Name == targetName {
			fmt.Println("found", targetName)
			file, err := f.Open()
			if err != nil {
				return psu.File{}, err
			}
			data, err := io.ReadAll(file)
			file.Close()
			if err != nil {
				return psu.File{}, err
			}

			return psu.File{
				Name:     "nhddl.elf",
				Created:  f.Modified,
				Modified: f.Modified,
				Data:     data,
			}, nil
		}
	}

	return psu.File{}, errors.New("target file not found")
}

const standaloneELF = "nhddl-standalone.elf"
const regularELF = "nhddl.elf"
