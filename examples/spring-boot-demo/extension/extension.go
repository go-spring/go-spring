package extension

import (
	"bytes"

	"github.com/go-spring/spring-base/conf"
	"github.com/spf13/viper"
)

func init() {
	conf.NewReader(func(b []byte) (map[string]interface{}, error) {
		v := viper.New()
		v.SetConfigType(".ini")
		if err := v.ReadConfig(bytes.NewBuffer(b)); err != nil {
			return nil, err
		}
		return v.AllSettings(), nil
	}, ".ini")
}
