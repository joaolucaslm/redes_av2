package fsm

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"
)

type Catalog struct {
	files map[string]File
	order []string
}

func NewCatalog(files ...File) (*Catalog, error) {
	c := &Catalog{files: make(map[string]File)}
	var problems []error
	for _, file := range files {
		if err := c.add(file); err != nil {
			problems = append(problems, err)
		}
	}
	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}
	return c, nil
}

func LoadCatalog(fsys fs.FS) (*Catalog, error) {
	names, err := fs.Glob(fsys, "*.json")
	if err != nil {
		return nil, err
	}
	slices.Sort(names)
	c := &Catalog{files: make(map[string]File)}
	var problems []error
	for _, name := range names {
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", name, err))
			continue
		}
		file, err := ParseFile(data)
		if err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", name, err))
			continue
		}
		if err := c.add(file); err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", name, err))
		}
	}
	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}
	return c, nil
}

func (c *Catalog) add(file File) error {
	if err := file.Validate(); err != nil {
		return err
	}
	if _, exists := c.files[file.Protocol]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateProtocol, file.Protocol)
	}
	c.files[file.Protocol] = file
	c.order = append(c.order, file.Protocol)
	return nil
}

func (c *Catalog) File(protocol string) (File, error) {
	file, ok := c.files[protocol]
	if !ok {
		return File{}, fmt.Errorf("%w: %s", ErrUnknownProtocol, protocol)
	}
	return file, nil
}

func (c *Catalog) Files() []File {
	files := make([]File, 0, len(c.order))
	for _, protocol := range c.order {
		files = append(files, c.files[protocol])
	}
	return files
}
