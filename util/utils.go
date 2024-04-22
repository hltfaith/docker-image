package util

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func DirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}

func FilesPath(path string, filesize int64) ([]string, error) {
	var filespath []string = []string{}
	err := filepath.Walk(path, func(subPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			if info.Size() == filesize {
				filespath = append(filespath, subPath)
			}
		}
		return err
	})
	return filespath, err
}

func FindDuplicates(data []string) int {
	m := make(map[string]bool)
	var duplicates []string
	for _, n := range data {
		if m[n] {
			duplicates = append(duplicates, n)
		} else {
			m[n] = true
		}
	}
	return len(duplicates)
}

// ref: github.com/docker/cli
func CreatedSince(created int64) string {
	var epoch = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	if created <= epoch {
		return "N/A"
	}
	v := HumanDuration(time.Now().UTC().Sub(time.Unix(created, 0)))
	return v + " ago"
}

// ref: github.com/docker/go-units
// HumanDuration returns a human-readable approximation of a duration
// (eg. "About a minute", "4 hours ago", etc.).
func HumanDuration(d time.Duration) string {
	if seconds := int(d.Seconds()); seconds < 1 {
		return "Less than a second"
	} else if seconds == 1 {
		return "1 second"
	} else if seconds < 60 {
		return fmt.Sprintf("%d seconds", seconds)
	} else if minutes := int(d.Minutes()); minutes == 1 {
		return "About a minute"
	} else if minutes < 60 {
		return fmt.Sprintf("%d minutes", minutes)
	} else if hours := int(d.Hours() + 0.5); hours == 1 {
		return "About an hour"
	} else if hours < 48 {
		return fmt.Sprintf("%d hours", hours)
	} else if hours < 24*7*2 {
		return fmt.Sprintf("%d days", hours/24)
	} else if hours < 24*30*2 {
		return fmt.Sprintf("%d weeks", hours/24/7)
	} else if hours < 24*365*2 {
		return fmt.Sprintf("%d months", hours/24/30)
	}
	return fmt.Sprintf("%d years", int(d.Hours())/24/365)
}

func ImageSize(size int64) string {
	return HumanSizeWithPrecision(float64(size), 3)
}

// ref: github.com/docker/go-units
// HumanSizeWithPrecision allows the size to be in any precision,
// instead of 4 digit precision used in units.HumanSize.
func HumanSizeWithPrecision(size float64, precision int) string {
	decimapAbbrs := []string{"B", "kB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"}
	size, unit := getSizeAndUnit(size, 1000.0, decimapAbbrs)
	return fmt.Sprintf("%.*g%s", precision, size, unit)
}

// ref: github.com/docker/go-units
func getSizeAndUnit(size float64, base float64, _map []string) (float64, string) {
	i := 0
	unitsLimit := len(_map) - 1
	for size >= base && i < unitsLimit {
		size = size / base
		i++
	}
	return size, _map[i]
}
