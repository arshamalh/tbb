package tbb

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/go-viper/mapstructure/v2"
	"github.com/joho/godotenv"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"
	"github.com/mcuadros/go-defaults"
)

// LoadConfig fills cfg, a pointer to a struct, in this order:
//
//  1. `default:"..."` struct tags,
//  2. environment variables, after loading envFiles (".env" when none are
//     given; missing files are ignored and real environment variables win),
//
// then validates it against `validate:"..."` tags.
//
// Fields are matched by their json tag; nested structs are separated by a
// double underscore and names are case-insensitive:
//
//	type Config struct {
//		tbb.Config             // embedded: TELEGRAM__TOKEN, DB__DSN, ...
//		AdminID int64 `json:"admin_id"` // ADMIN_ID
//	}
//
//	var cfg Config
//	if err := tbb.LoadConfig(&cfg); err != nil { ... }
func LoadConfig(cfg any, envFiles ...string) error {
	if len(envFiles) == 0 {
		envFiles = []string{".env"}
	}
	for _, f := range envFiles {
		if err := godotenv.Load(f); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("load %s: %w", f, err)
		}
	}

	defaults.SetDefaults(cfg)

	k := koanf.New(".")
	if err := k.Load(env.Provider("", "__", strings.ToLower), nil); err != nil {
		return fmt.Errorf("read environment: %w", err)
	}
	err := k.UnmarshalWithConf("", cfg, koanf.UnmarshalConf{
		Tag: "json",
		DecoderConfig: &mapstructure.DecoderConfig{
			DecodeHook: mapstructure.ComposeDecodeHookFunc(
				mapstructure.StringToTimeDurationHookFunc(),
				mapstructure.TextUnmarshallerHookFunc(),
			),
			WeaklyTypedInput: true,
			Squash:           true, // embedded structs share the parent's namespace
			TagName:          "json",
			Result:           cfg,
		},
	})
	if err != nil {
		return fmt.Errorf("decode config: %w", err)
	}

	if err := validator.New().Struct(cfg); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	return nil
}
