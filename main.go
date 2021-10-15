package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type timecode struct {
	Time  string
	Title string
}

type track struct {
	Number int
	Total  int
	Title  string
	Start  string
	End    string
	Artist string
	Album  string
}

func parseTime(t string) error {
	_, err := time.Parse("15:04:05", strings.Trim(t, " "))
	return err
}

func (t *track) outputFilename(audioFile string) string {
	padFmt := "%02d - %v%v"
	if t.Total > 99 {
		padFmt = "%03d - %v%v"
	}

	v := fmt.Sprintf(
		padFmt,
		t.Number,
		t.Title,
		filepath.Ext(audioFile),
	)
	return path.Join(t.Artist, t.Album, v)
}

func (t *track) ffmpegArgs(audioFile string) []string {
	args := []string{
		"-nostdin",
		"-y",
		"-loglevel",
		"error",
	}

	if t.End == "" {
		// We're on the last track so read to EOF
		args = append(args, []string{
			"-ss", t.Start}...)
	} else {
		// Read from start to end
		args = append(args, []string{
			"-ss", t.Start, "-to", t.End}...)
	}

	args = append(args, []string{
		"-i",
		fmt.Sprintf("%v", audioFile),
		"-vn", "-c", "copy", "-f", "mp3",
		t.outputFilename(audioFile),
	}...)

	return args
}

func (t *track) eyeD3Args(audioFile string) []string {
	return []string{
		fmt.Sprintf("%v=\"%v\"", "--artist", t.Artist),
		fmt.Sprintf("%v=\"%v\"", "--album-artist", t.Artist),
		fmt.Sprintf("%v=\"%v\"", "--album", t.Album),
		fmt.Sprintf("%v=\"%v\"", "--title", t.Title),
		fmt.Sprintf("%v=%v", "--track", t.Number),
		fmt.Sprintf("%v=%v", "--track-total", t.Total),
		t.outputFilename(audioFile),
	}
}

func execCommand(c string, arg ...string) error {
	cmd := exec.Command(c, arg...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Start()
	if err != nil {
		return err
	}

	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf(stderr.String())
	}

	return nil
}

func run(audioFile, timecodesFile, artist, album string) error {
	_, err := os.Stat(audioFile)
	if err != nil {
		return fmt.Errorf("audio file not found")
	}

	_, err = os.Stat(timecodesFile)
	if err != nil {
		return fmt.Errorf("timecodes file not found")
	}

	f, err := os.Open(timecodesFile)
	if err != nil {
		return fmt.Errorf("cannot read timecodes file")
	}
	defer f.Close()

	s := bufio.NewScanner(f)

	var timecodes [][]string
	for s.Scan() {
		if s.Text() == "" {
			continue
		}

		tc := strings.SplitAfterN(s.Text(), " ", 2)
		if len(tc) < 2 {
			return fmt.Errorf("invalid format")
		}

		if err := parseTime(tc[0]); err != nil {
			return fmt.Errorf("invalid timecode")
		}

		tc[0] = strings.Trim(tc[0], " ")
		tc[1] = strings.Trim(tc[1], " ")

		timecodes = append(timecodes, tc)
	}

	if len(timecodes) == 0 {
		return fmt.Errorf("no timecodes found")
	}

	if len(timecodes) > 999 {
		return fmt.Errorf("too many tracks: %d", len(timecodes))
	}

	var tracks []track

	for i := range timecodes {
		t := track{
			Number: i + 1,
			Title:  timecodes[i][1],
			Start:  timecodes[i][0],
			Artist: artist,
			Album:  album,
			Total:  len(timecodes),
		}
		tracks = append(tracks, t)

		if i == 1 {
			tracks[i-1].End = timecodes[i][0]
		}

		if i > 0 && i < len(timecodes)-1 {
			tracks[i].End = timecodes[i+1][0]
		}
	}

	err = os.MkdirAll(path.Join(artist, album), 0700)
	if err != nil {
		return err
	}

	for _, t := range tracks {
		fmt.Printf("processing track \"%v\"\n", t.outputFilename(audioFile))
		err := execCommand("ffmpeg", t.ffmpegArgs(audioFile)...)
		if err != nil {
			return err
		}

		err = execCommand("eyed3", t.eyeD3Args(audioFile)...)
		if err != nil {
			return err
		}
	}

	return nil
}

func main() {
	filename := flag.String("filename", "", "Path to the audio file")
	timecodes := flag.String("timecodes", "", "Path to the timecodes file")
	artist := flag.String("artist", "", "Album artist")
	album := flag.String("album", "", "Album name")

	flag.Parse()

	if *filename == "" || *timecodes == "" || *artist == "" || *album == "" {
		flag.Usage()
		os.Exit(1)
	}

	if err := run(*filename, *timecodes, *artist, *album); err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}
