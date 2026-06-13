package config

import "os"

func sysLookupEnv(key string) (string, bool) {
	return os.LookupEnv(key)
}
