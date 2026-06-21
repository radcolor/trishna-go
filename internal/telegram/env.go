package telegram

import "os"

func sysLookupEnv(key string) (string, bool) {
	return os.LookupEnv(key)
}
