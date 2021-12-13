package disk

import (
	"io/ioutil"
	"sort"
	"strings"
)

const (
	snapExt = ".snap"
	walExt  = ".wal"
	format  = "%016x-%016x"
)

func list(path, ext string) ([]string, error) {
	ls, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, f := range ls {
		if strings.HasSuffix(f.Name(), ext) {
			files = append(files, f.Name())
		}
	}

	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	return files, nil
}
