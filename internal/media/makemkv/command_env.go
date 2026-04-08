package makemkv

import (
	"os"
	"strings"
)

const faketimeLibraryPath = "/usr/local/lib/libfaketime.so.1"

func CommandEnv(base []string) []string {
	env := overrideEnv(base, "HOME", "/config")

	faketime := strings.TrimSpace(os.Getenv("FAKETIME"))
	if faketime == "" {
		env = removeEnv(env, "LD_PRELOAD", "FAKETIME", "FAKETIME_DONT_FAKE_MONOTONIC")
		return env
	}

	env = overrideEnv(env,
		"LD_PRELOAD", faketimeLibraryPath,
		"FAKETIME", faketime,
		"FAKETIME_DONT_FAKE_MONOTONIC", "1",
	)
	return env
}

func overrideEnv(base []string, keyValues ...string) []string {
	env := append([]string{}, base...)
	for i := 0; i+1 < len(keyValues); i += 2 {
		key := keyValues[i]
		prefix := key + "="
		value := keyValues[i+1]
		replaced := false
		for index, entry := range env {
			if strings.HasPrefix(entry, prefix) {
				env[index] = prefix + value
				replaced = true
			}
		}
		if !replaced {
			env = append(env, prefix+value)
		}
	}
	return env
}

func removeEnv(base []string, keys ...string) []string {
	filtered := make([]string, 0, len(base))
	for _, entry := range base {
		remove := false
		for _, key := range keys {
			if strings.HasPrefix(entry, key+"=") {
				remove = true
				break
			}
		}
		if !remove {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}
