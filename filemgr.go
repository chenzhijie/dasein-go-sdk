package dasein_go_sdk

import "sync"

type ReadFileMgr struct {
	fileSliceId map[string]uint64
	lock        sync.RWMutex
}

func NewReadFileMgr() *ReadFileMgr {
	return &ReadFileMgr{
		fileSliceId: make(map[string]uint64, 0),
	}
}

func (rfm *ReadFileMgr) IncreAndGetSliceId(fileHashStr string) uint64 {
	rfm.lock.Lock()
	defer rfm.lock.Unlock()
	id := rfm.fileSliceId[fileHashStr]
	id++
	rfm.fileSliceId[fileHashStr] = id
	return id
}

func (rfm *ReadFileMgr) RemoveSliceId(fileHashStr string) {
	rfm.lock.Lock()
	defer rfm.lock.Unlock()
	delete(rfm.fileSliceId, fileHashStr)
}
