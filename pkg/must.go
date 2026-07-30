package pkg

import "os"

func Must(err error) {
	if err != nil {
		os.Exit(-1)
	}
}
