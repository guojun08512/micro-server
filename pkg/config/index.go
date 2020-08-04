package config

import (
	"fmt"
	"path"
	"strings"

	"github.com/spf13/viper"
)

// Config is a singleton config manager
var Config = func() *viper.Viper {
	config := viper.New()
	config.SetEnvPrefix("keyayun")
	config.SetConfigName("config")
	config.SetConfigType("yml")
	paths := []string{".", "..", path.Join("..", "..")}
	var err error
	for _, p := range paths {
		config.AddConfigPath(p)
		err = config.ReadInConfig()
		if err == nil {
			break
		}
	}
	if err != nil {
		panic(fmt.Errorf("Fatal error config file: %s", err))
	}
	config.AutomaticEnv()
	replacer := strings.NewReplacer(".", "_")
	config.SetEnvKeyReplacer(replacer)
	return config
}()
