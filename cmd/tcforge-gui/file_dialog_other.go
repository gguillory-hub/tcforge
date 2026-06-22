//go:build gui && !windows

package main

import nativedialog "github.com/sqweek/dialog"

func openMediaFiles() ([]string, error) {
	path, err := nativedialog.File().Title("Add Media File").Filter("Media files", "mp4", "mov", "m4v", "mxf").Load()
	if err != nil {
		return nil, err
	}
	return []string{path}, nil
}
