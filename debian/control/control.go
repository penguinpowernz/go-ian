package control

import (
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"

	"github.com/fatih/structs"
	"github.com/penguinpowernz/go-ian/util/str"
)

// Control represents a debian control file
type Control struct {
	Name       string   `dpkg:"Package"`
	Version    string   `dpkg:"Version"`
	Section    string   `dpkg:"Section"`
	Priority   string   `dpkg:"Priority"`
	Arch       string   `dpkg:"Architecture"`
	Maintainer string   `dpkg:"Maintainer"`
	Essential  string   `dpkg:"Essential"`
	Homepage   string   `dpkg:"Homepage"`
	Size       string   `dpkg:"Installed-Size"`
	Depends    []string `dpkg:"Depends,omitempty"`
	Conflicts  []string `dpkg:"Conflicts,omitempty"`
	Provides   []string `dpkg:"Provides,omitempty"`
	Replaces   []string `dpkg:"Replaces,omitempty"`
	Desc       string   `dpkg:"Description"`
	LongDesc   string   `dpkg:"Text"`
}

// Filename returns the name of the packages filename
func (c Control) Filename() string {
	return fmt.Sprintf("%s_%s_%s.deb", c.Name, c.Version, c.Arch)
}

// WriteFile will write the control file to the given filename
func (c Control) WriteFile(fn string) error {
	data := []byte(c.String())
	return ioutil.WriteFile(fn, data, 0755)
}

// String returns the text contents of the control file
func (c Control) String() string {
	lines := []string{}

	s := structs.New(c)
	for _, f := range s.Fields() {

		// we always want this to come last
		if f.Name() == "LongDesc" {
			continue
		}

		tag := f.Tag("dpkg")
		bits := strings.Split(tag, ",")
		omitempty := strings.Contains(tag, ",omitempty")
		name := bits[0]
		value := ""

		if f.IsZero() && omitempty {
			continue
		}

		ok := false

		// turn the slices into strings
		switch f.Kind() {
		case reflect.Slice:
			var slice []string
			slice, ok = f.Value().([]string)
			if !ok {
				continue
			}
			value = serialize(slice)
		case reflect.String:
			value, ok = f.Value().(string)
		}

		if !ok {
			continue
		}

		lines = append(lines, name+": "+value)
	}

	if c.LongDesc == "" {
		c.LongDesc = c.Desc
	}

	for _, l := range strings.Split(c.LongDesc, "\n") {
		lines = append(lines, "  "+l)
	}

	return strings.Join(lines, "\n")
}

// Default returns a default control file, taking an optional
// argument for the name
func Default(name ...string) Control {
	n := "my-package"
	if len(name) > 0 {
		n = name[0]
	}

	return Control{
		Name:      n,
		Version:   "0.0.1",
		Arch:      "all",
		Section:   "misc",
		Essential: "no",
		Priority:  "optional",
		Homepage:  "http://example.com",
		Desc:      "This is a description",
		LongDesc:  "This is a longer description\nthat takes up multiple lines",
	}
}

// Parse takes a control file contents and turns it into
// a control file struct.
func Parse(ctrl string) (Control, error) {
	lines := strings.Split(ctrl, "\n")
	longDesc := []string{}

	c := Default()

	msi := map[string]interface{}{}

	for _, line := range lines {
		if strings.HasPrefix(line, "  ") {
			longDesc = append(longDesc, strings.TrimSpace(line))
			continue
		}
		if strings.TrimSpace(line) == "" {
			continue
		}

		bits := strings.Split(line, ":")
		name := strings.TrimSpace(bits[0])
		value := strings.TrimSpace(strings.Join(bits[1:], ":"))

		msi[name] = value
	}

	s := structs.New(&c)

	for _, f := range s.Fields() {
		key := f.Tag("dpkg")
		key = strings.Split(key, ",")[0]

		val, ok := msi[key]
		if !ok {
			continue
		}

		if f.Kind() == reflect.Slice {
			val = unserialize(val.(string))
		}

		err := f.Set(val)
		if err != nil {
			return c, err
		}
	}

	c.LongDesc = strings.Join(str.CleanStrings(longDesc), "\n")

	return c, nil
}

// Read will read the given filename, parse it's contents and
// return a Control struct.
func Read(fn string) (Control, error) {
	data, err := ioutil.ReadFile(fn)
	if err != nil {
		return Control{}, err
	}

	return Parse(string(data))
}

func serialize(strs []string) string {
	return strings.Join(strs, ", ")
}

func unserialize(s string) []string {
	return str.CleanStrings(strings.Split(s, ","))
}
