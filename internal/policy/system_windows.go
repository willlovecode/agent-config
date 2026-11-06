package policy

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows/registry"
)

func readSystemPolicy() ([]byte, string, error) {
	const key = `SOFTWARE\Policies\agtk`
	const source = `HKLM\` + key + `\Policy`
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, key, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if errors.Is(err, registry.ErrNotExist) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("read company policy at %s: %w", source, err)
	}
	defer k.Close()
	data, kind, err := k.GetStringValue("Policy")
	if errors.Is(err, registry.ErrNotExist) {
		return nil, "", nil
	}
	if err != nil || kind != registry.SZ {
		return nil, "", fmt.Errorf("company policy at %s needs a REG_SZ value", source)
	}
	return []byte(data), source, nil
}
