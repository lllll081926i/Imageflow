package batch

import (
	"reflect"

	"github.com/lllll081926i/imageflow-go/internal/settings"
)

func mergeSettings(current *settings.Settings, base, edited settings.Settings) {
	dst, before, after := reflect.ValueOf(current).Elem(), reflect.ValueOf(base), reflect.ValueOf(edited)
	for i := 0; i < dst.NumField(); i++ {
		if !reflect.DeepEqual(before.Field(i).Interface(), after.Field(i).Interface()) {
			dst.Field(i).Set(after.Field(i))
		}
	}
}
