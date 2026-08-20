package grpc

import (
	"strings"

	"connectrpc.com/connect"
)

func ExtractReadMask[T any](req *connect.Request[T]) []string {
	rawMask := req.Header().Get("x-exonex-read-mask")
	if rawMask == "" || rawMask == "*" {
		return nil
	}
	rawPaths := strings.Split(rawMask, ",")
	paths := make([]string, 0, len(rawPaths))
	for _, p := range rawPaths {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			paths = append(paths, trimmed)
		}
	}
	return rawPaths
}
