// download the soundcloud tracks
package soundcloud

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"sync"

	m "github.com/grafov/m3u8"
	bar "github.com/schollz/progressbar/v3"
)

// expand the given path ~/Desktop to the current logged in user /home/<username>/Desktop
func expandPath(path string) (string, error) {
	if len(path) == 0 || path[0] != '~' {
		return path, nil
	}

	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	return filepath.Join(usr.HomeDir, path[1:]), nil

}

// validate the given path
// and check if the file already exists or not
func fileExists(path string) bool {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return false
	}
	return true
}

// extract the urls of the individual segment and then steam/download.
func downloadSeg(wg *sync.WaitGroup, segmentURI string, file *os.File, dlbar *bar.ProgressBar) {
	defer wg.Done()
	resp, err := http.Get(segmentURI)

	if err != nil {
		return
	}

	defer resp.Body.Close()

	// append to the file
	if dlbar == nil {
		_, err = io.Copy(io.MultiWriter(file), resp.Body)
	} else {
		_, err = io.Copy(io.MultiWriter(file, dlbar), resp.Body)
	}

	if err != nil {
		return
	}

}

// extract the urls of the individual segment and then steam/download.
func downloadSegBuffer(wg *sync.WaitGroup, segmentURI string, buf *bytes.Buffer) {
	defer wg.Done()
	resp, err := http.Get(segmentURI)

	if err != nil {
		return
	}

	defer resp.Body.Close()

	_, err = io.Copy(buf, resp.Body)

	if err != nil {
		return
	}
}

func getSegments(body io.Reader) []string {
	segments := make([]string, 0)
	pl, listType, err := m.DecodeFrom(body, true)

	if err != nil {
		return nil
	}

	switch listType {
	case m.MEDIA:
		mediapl := pl.(*m.MediaPlaylist)
		for _, segment := range mediapl.Segments {
			if segment == nil {
				continue
			}
			segments = append(segments, segment.URI)
		}
	}
	return segments
}

// using the goroutine to download each segment concurrently and wait till all finished
func DownloadM3u8(filepath string, dlbar *bar.ProgressBar, segments []string) error {

	file, _ := os.OpenFile(filepath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)

	// the go routine now
	var wg sync.WaitGroup

	for _, segment := range segments {
		wg.Add(1)
		downloadSeg(&wg, segment, file, dlbar)
	}

	return nil
}

func DownloadM3u8ToBuffer(buf *bytes.Buffer, segments []string) error {
	var wg sync.WaitGroup

	for _, segment := range segments {
		wg.Add(1)
		downloadSegBuffer(&wg, segment, buf)
	}

	return nil
}

// before download validation
// return the path if everything is alright.
func validateDownload(dlpath string, trackName string) string {

	testPath := path.Join(dlpath, trackName)
	path, err := expandPath(testPath)

	// TODO: handle all different kind of errors
	if fileExists(path) || err != nil {
		return ""
	}
	return path
}

// download the track
func Download(track DownloadTrack, dlpath string) string {
	// TODO: Prompt Y/N if the file exists and rename by adding _<random/date>.<ext>
	trackName := track.SoundData.Title + "[" + track.Quality + "]." + track.Ext
	path := validateDownload(dlpath, trackName)

	// check if the track is hls
	if track.Quality != "low" {

		resp, err := http.Get(track.Url)
		if err != nil {
			return ""
		}
		defer resp.Body.Close()

		dlbar := bar.DefaultBytes(
			resp.ContentLength,
			"Downloading",
		)
		segments := getSegments(resp.Body)
		DownloadM3u8(path, dlbar, segments)

		return path
	}
	resp, err := http.Get(track.Url)

	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	// check if the file exists
	f, _ := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()

	bar := bar.DefaultBytes(
		resp.ContentLength,
		"Downloading",
	)

	io.Copy(io.MultiWriter(f, bar), resp.Body)

	return path
}

func DownloadToBuffer(track DownloadTrack, buf *bytes.Buffer) error {
	resp, err := http.Get(track.Url)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	segments := getSegments(resp.Body)
	DownloadM3u8ToBuffer(buf, segments)

	return nil
}