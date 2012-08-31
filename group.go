package flags

import (
	"errors"
	"fmt"
	"reflect"
	"unicode/utf8"
)

// The provided container is not a pointer to a struct
var ErrNotPointerToStruct = errors.New("provided data is not a pointer to struct")

// The provided short name is longer than a single character
var ErrShortNameTooLong = errors.New("short names can only be 1 character")

// Option flag information. Contains a description of the option, short and
// long name as well as a default value and whether an argument for this
// flag is optional.
type Info struct {
	// The short name of the option (a single character). If not 0, the
	// option flag can be 'activated' using -<ShortName>. Either ShortName
	// or LongName needs to be non-empty.
	ShortName        rune

	// The long name of the option. If not "", the option flag can be
	// activated using --<LongName>. Either ShortName or LongName needs
	// to be non-empty.
	LongName         string

	// The description of the option flag. This description is shown
	// automatically in the builtin help.
	Description      string

	// The default value of the option. The default value is used when
	// the option flag is marked as having an OptionalArgument. This means
	// that when the flag is specified, but no option argument is given,
	// the value of the field this option represents will be set to
	// Default. This is only valid for non-boolean options.
	Default          string

	// If true, specifies that the argument to an option flag is optional.
	// When no argument to the flag is specified on the command line, the
	// value of Default will be set in the field this option represents.
	// This is only valid for non-boolean options.
	OptionalArgument bool

	value   reflect.Value
	options reflect.StructTag
}

// An option group. The option group has a name and a set of options.
type Group struct {
	// The name of the group.
	Name       string

	// A map of long names to option info descriptions.
	LongNames  map[string]*Info

	// A map of short names to option info descriptions.
	ShortNames map[rune]*Info

	// A list of all the options in the group.
	Options    []*Info

	// An error which occurred when creating the group.
	Error error

	data  interface{}
}

func (info *Info) canArgument() bool {
	if info.isBool() {
		return false
	}

	if info.isFunc() {
		return (info.value.Type().NumIn() > 0)
	}

	return true
}

func (info *Info) isBool() bool {
	tp := info.value.Type()

	switch tp.Kind() {
	case reflect.Bool:
		return true
	case reflect.Slice:
		return (tp.Elem().Kind() == reflect.Bool)
	}

	return false
}

func (info *Info) isFunc() bool {
	return info.value.Type().Kind() == reflect.Func
}

func (info *Info) call(value *string) {
	if value == nil {
		info.value.Call(nil)
	} else {
		val := reflect.New(reflect.TypeOf(*value))
		val.SetString(*value)

		info.value.Call([]reflect.Value {reflect.Indirect(val)})
	}
}

// Set the value of an option to the specified value. An error will be returned
// if the specified value could not be converted to the corresponding option
// value type.
func (info *Info) Set(value *string) error {
	if info.isFunc() {
		info.call(value)
	} else if value != nil {
		return convert(*value, info.value, info.options)
	} else {
		return convert("", info.value, info.options)
	}

	return nil
}

// Convert an option to a human friendly readable string describing the option.
func (info *Info) String() string {
	var s string
	var short string

	if info.ShortName != 0 {
		data := make([]byte, utf8.RuneLen(info.ShortName))
		utf8.EncodeRune(data, info.ShortName)
		short = string(data)

		if len(info.LongName) != 0 {
			s = fmt.Sprintf("-%s, --%s", short, info.LongName)
		} else {
			s = fmt.Sprintf("-%s", short)
		}
	} else if len(info.LongName) != 0 {
		s = fmt.Sprintf("--%s", info.LongName)
	}

	if len(info.Description) != 0 {
		return fmt.Sprintf("%s (%s)", s, info.Description)
	}

	return s
}

// NewGroup creates a new option group with a given name and underlying data
// container. The data container is a pointer to a struct. The fields of the
// struct represent the command line options (using field tags) and their values
// will be set when their corresponding options appear in the command line
// arguments.
func NewGroup(name string, data interface{}) *Group {
	ret := &Group{
		Name:       name,
		LongNames:  make(map[string]*Info),
		ShortNames: make(map[rune]*Info),
		data:       data,
	}

	ret.Error = ret.scan()
	return ret
}

func (g *Group) scan() error {
	// Get all the public fields in the data struct
	ptrval := reflect.ValueOf(g.data)

	if ptrval.Type().Kind() != reflect.Ptr {
		return ErrNotPointerToStruct
	}

	stype := ptrval.Type().Elem()

	if stype.Kind() != reflect.Struct {
		return ErrNotPointerToStruct
	}

	realval := reflect.Indirect(ptrval)

	for i := 0; i < stype.NumField(); i++ {
		field := stype.Field(i)

		// PkgName is set only for non-exported fields, which we ignore
		if field.PkgPath != "" {
			continue
		}

		// Skip anonymous fields
		if field.Anonymous {
			continue
		}

		// Skip fields with the no-flag tag
		if field.Tag.Get("no-flag") != "" {
			continue
		}

		longname := field.Tag.Get("long")
		shortname := field.Tag.Get("short")

		if longname == "" && shortname == "" {
			continue
		}

		short := rune(0)
		rc := utf8.RuneCountInString(shortname)

		if rc > 1 {
			return ErrShortNameTooLong
		} else if rc == 1 {
			short, _ = utf8.DecodeRuneInString(shortname)
		}

		description := field.Tag.Get("description")
		def := field.Tag.Get("default")

		optional := (field.Tag.Get("optional") != "")

		info := &Info{
			Description:      description,
			ShortName:        short,
			LongName:         longname,
			Default:          def,
			OptionalArgument: optional,
			value:            realval.Field(i),
			options:          field.Tag,
		}

		g.Options = append(g.Options, info)

		if info.ShortName != 0 {
			g.ShortNames[info.ShortName] = info
		}

		if info.LongName != "" {
			g.LongNames[info.LongName] = info
		}
	}

	return nil
}
