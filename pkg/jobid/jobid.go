package jobid

import (
	"fmt"

	"github.com/google/uuid"
)

const sep = "|"

func GenKey(url, preset string) string {
	return fmt.Sprintf("%s%s%s", url, sep, preset)
}

func GenUUIDv5(url, preset string) string {
	key := GenKey(url, preset)
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(key)).String()
}
