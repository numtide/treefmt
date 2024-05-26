package cli

import (
	"fmt"
	"reflect"

	"github.com/alecthomas/kong"
	"github.com/charmbracelet/log"
)

var Options []kong.Option

func init() {
	Options = []kong.Option{
		kong.TypeMapper(reflect.TypeOf(log.DebugLevel), logLevelDecoder()),
	}
}

func logLevelDecoder() kong.MapperFunc {
	return func(ctx *kong.DecodeContext, target reflect.Value) error {
		t, err := ctx.Scan.PopValue("string")
		if err != nil {
			return err
		}
		var str string
		switch v := t.Value.(type) {
		case string:
			str = v
		default:
			return fmt.Errorf("expected a string but got %q (%T)", t, t.Value)
		}
		level, err := log.ParseLevel(str)
		if err != nil {
			return fmt.Errorf("failed to parse '%v' as log level: %w", level, err)
		}
		target.Set(reflect.ValueOf(level))
		return nil
	}
}
