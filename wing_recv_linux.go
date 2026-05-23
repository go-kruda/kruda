//go:build linux

package kruda

func submitIdleRecv(_ engine, _ int32, _ []byte, _ int) {
	// Linux Wing uses persistent edge-triggered EPOLLIN. After a full direct
	// write there is no EPOLLOUT bit to remove, so re-submitting EPOLLIN would
	// only issue a redundant epoll_ctl on the hot path.
}
