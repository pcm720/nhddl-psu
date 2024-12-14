package gh

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"slices"
	"time"

	"github.com/pcm720/nhddl-psu/gh/internal/fetch"
	"github.com/pcm720/psu-go"
)

type Fetcher struct {
	Repo      string
	CORSProxy string
}

type GHRelease struct {
	Assets []struct {
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

type GHTag struct {
	Name string `json:"name"`
}

func (g *Fetcher) getReleaseURL(tag string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := fetch.Fetch(ctx, "https://api.github.com/repos/"+g.Repo+"/releases/tags/"+tag)
	if err != nil {
		return "", err
	}

	release := GHRelease{}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	if len(release.Assets) < 1 {
		return "", errors.New("no assets")
	}

	return release.Assets[0].BrowserDownloadURL, nil
}

func (g *Fetcher) GetAllTags() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := fetch.Fetch(ctx, "https://api.github.com/repos/"+g.Repo+"/tags")
	if err != nil {
		return nil, err
	}

	tags := []GHTag{}
	if err := json.NewDecoder(resp.Body).Decode(&tags); (err != nil) && (err != io.EOF) {
		return nil, err
	}

	tagNames := make([]string, len(tags))
	for i, tag := range tags {
		tagNames[i] = tag.Name
	}

	return tagNames, nil
}

// Downloads files from the first GitHub release asset ZIP
func (g *Fetcher) GetFiles(tag string, targetFiles []string) ([]psu.File, error) {
	fmt.Println("getting release ZIP for", tag)
	rel, err := g.getReleaseURL(tag)
	if err != nil {
		return nil, err
	}

	rel = g.CORSProxy + rel
	fmt.Println("downloading", rel)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	resp, err := fetch.Fetch(ctx, rel)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("invalid status code %d", resp.StatusCode)
	}

	zipData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	fmt.Println("opening file", rel)
	z, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, err
	}

	fmt.Println("processing ZIP archive")
	out := make([]psu.File, 0, len(targetFiles))
	for _, f := range z.File {
		if !f.FileInfo().IsDir() && slices.Contains(targetFiles, f.Name) {
			fmt.Println("adding", f.Name)

			file, err := f.Open()
			if err != nil {
				return nil, err
			}
			data, err := io.ReadAll(file)
			file.Close()
			if err != nil {
				return nil, err
			}

			out = append(out, psu.File{
				Name:     path.Base(f.Name),
				Created:  f.Modified,
				Modified: f.Modified,
				Data:     data,
			})
		}
	}
	if len(out) == 0 {
		return nil, errors.New("no files found")
	}
	return out, nil
}
