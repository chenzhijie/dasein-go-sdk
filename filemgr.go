package dasein_go_sdk

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"sync"
	"time"
)

const (
	STORE_FILES_DIR    = "./stores/"    // temp directory, using for keep sending files infomation
	DOWNLOAD_FILES_DIR = "./downloads/" // downloads file directory, using for store downloaded files
)

type FileMgr struct {
	StoreFileMgr
	ReadFileMgr
}

// NewFileMgr init a new file manager
func NewFileMgr() *FileMgr {
	fm := &FileMgr{}
	fm.fileSliceId = make(map[string]uint64, 0)
	fm.fileInfos = make(map[string]*fmFileInfo, 0)
	return fm
}

// fileBlockInfo record a block infomation of a file
// including block hash string, a timestamp of sending this block to remote peer
// a timestamp of receiving a block from remote peer
type fmBlockInfo struct {
	Hash          string `json:"hash"`
	SendTimestamp int64  `json:"sendts"`
	RecvTimestamp int64  `json:"recvts"`
}

// NewFmBlockInfo new file manager block info
func NewFmBlockInfo(hash string) *fmBlockInfo {
	return &fmBlockInfo{
		Hash: hash,
	}
}

// fmFileInfo keep all blocks infomation and the prove private key for generating tags
type fmFileInfo struct {
	Blocks       []*fmBlockInfo `json:"blocks"`
	ProvePrivKey []byte         `json:"prove_private_key"`
}

// NewFmFileInfo new fm file info
func NewFmFileInfo(provePrivKey []byte) *fmFileInfo {
	blks := make([]*fmBlockInfo, 0)
	return &fmFileInfo{
		Blocks:       blks,
		ProvePrivKey: provePrivKey,
	}
}

type StoreFileMgr struct {
	lock      sync.RWMutex           // Lock
	fileInfos map[string]*fmFileInfo // FileName <=> FileInfo
}

// NewStoreFile init store file info
func (sfm *StoreFileMgr) NewStoreFile(hashStr string, provePrivKey []byte) error {
	sfm.lock.Lock()
	defer sfm.lock.Unlock()
	if _, err := os.Stat(STORE_FILES_DIR); os.IsNotExist(err) {
		err = os.MkdirAll(STORE_FILES_DIR, 0755)
		if err != nil {
			return err
		}
	}

	data, _ := ioutil.ReadFile(sfm.storeFilePath(hashStr))
	if len(data) == 0 {
		sfm.fileInfos[hashStr] = NewFmFileInfo(provePrivKey)
		buf, err := json.Marshal(sfm.fileInfos[hashStr])
		if err != nil {
			return err
		}
		return ioutil.WriteFile(sfm.storeFilePath(hashStr), buf, 0666)
	}

	fi := &fmFileInfo{}
	err := json.Unmarshal(data, fi)
	if err != nil {
		return err
	}
	sfm.fileInfos[hashStr] = fi
	return nil
}

func (sfm *StoreFileMgr) DelStoreFileInfo(hashStr string) error {
	sfm.lock.Lock()
	defer sfm.lock.Unlock()
	delete(sfm.fileInfos, hashStr)
	return os.Remove(sfm.storeFilePath(hashStr))
}

// AddStoredBlock add a new stored block info to map and local storage
func (sfm *StoreFileMgr) AddStoredBlock(hashStr, blockHash string) error {
	sfm.lock.Lock()
	defer sfm.lock.Unlock()
	fi, ok := sfm.fileInfos[hashStr]
	if !ok {
		return errors.New("file info not found")
	}
	oldBlks := fi.Blocks
	newBlks := fi.Blocks
	for _, blk := range oldBlks {
		if blk.Hash == blockHash {
			return nil
		}
	}
	newBlk := NewFmBlockInfo(blockHash)
	newBlk.SendTimestamp = time.Now().Unix()
	newBlks = append(newBlks, newBlk)
	fi.Blocks = newBlks
	buf, err := json.Marshal(fi)
	if err != nil {
		fi.Blocks = oldBlks
		return err
	}
	return ioutil.WriteFile(sfm.storeFilePath(hashStr), buf, 0666)
}

// IsBlockStored check if block has stored
func (sfm *StoreFileMgr) IsBlockStored(hashStr string, blockHash string) bool {
	sfm.lock.RLock()
	defer sfm.lock.RUnlock()
	fi, ok := sfm.fileInfos[hashStr]
	if !ok || len(fi.Blocks) == 0 {
		return false
	}
	for _, b := range fi.Blocks {
		if b.Hash == blockHash {
			return true
		}
	}
	return false
}

func (sfm *StoreFileMgr) StoredBlockCount(hashStr string) int {
	sfm.lock.RLock()
	defer sfm.lock.RUnlock()
	fi := sfm.fileInfos[hashStr]
	if fi == nil {
		return 0
	}
	return len(fi.Blocks)
}

func (sfm *StoreFileMgr) GetFileProvePrivKey(hashStr string) []byte {
	sfm.lock.RLock()
	defer sfm.lock.RUnlock()
	fi := sfm.fileInfos[hashStr]
	if fi == nil {
		return nil
	}
	return fi.ProvePrivKey
}

// OnSendAllBlocks clean data after sending all blocks
func (sfm *StoreFileMgr) OnSendAllBlocks(hashStr string) {
	sfm.lock.Lock()
	defer sfm.lock.Unlock()
	delete(sfm.fileInfos, hashStr)
}

func (sfm *StoreFileMgr) storeFilePath(hashStr string) string {
	return STORE_FILES_DIR + hashStr
}

type ReadFileMgr struct {
	fileSliceId map[string]uint64
	lock        sync.RWMutex
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
