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
	STORE_FILES_DIR    = "./stores/"      // temp directory, using for keep sending files infomation
	DOWNLOAD_FILES_DIR = "./downloading/" // downloads file directory, using for store temp downloaded files
)

type FileMgr struct {
	StoreFileMgr
	ReadFileMgr
}

// NewFileMgr init a new file manager
func NewFileMgr() *FileMgr {
	fm := &FileMgr{}
	fm.readFileInfos = make(map[string]*fmFileInfo, 0)
	fm.fileInfos = make(map[string]*fmFileInfo, 0)
	return fm
}

// fileBlockInfo record a block infomation of a file
// including block hash string, a timestamp of sending this block to remote peer
// a timestamp of receiving a block from remote peer
type fmBlockInfo struct {
	Hash           string   `json:"hash"`
	SendTimestamp  int64    `json:"sendts"`
	RecvTimestamp  int64    `json:"recvts"`
	NodeAddr       string   `json:"node_address"`
	NodeWalletAddr string   `json:"node_wallet_address"`
	NodeList       []string `json:"node_list"`
	ChildHashStrs  []string `json:"childs"`
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
func (sfm *StoreFileMgr) NewStoreFile(fileHashStr string, provePrivKey []byte) error {
	sfm.lock.Lock()
	defer sfm.lock.Unlock()
	if _, err := os.Stat(STORE_FILES_DIR); os.IsNotExist(err) {
		err = os.MkdirAll(STORE_FILES_DIR, 0755)
		if err != nil {
			return err
		}
	}

	data, _ := ioutil.ReadFile(storeFilePath(fileHashStr))
	if len(data) == 0 {
		sfm.fileInfos[fileHashStr] = NewFmFileInfo(provePrivKey)
		buf, err := json.Marshal(sfm.fileInfos[fileHashStr])
		if err != nil {
			return err
		}
		return ioutil.WriteFile(storeFilePath(fileHashStr), buf, 0666)
	}

	fi := &fmFileInfo{}
	err := json.Unmarshal(data, fi)
	if err != nil {
		return err
	}
	sfm.fileInfos[fileHashStr] = fi
	return nil
}

func (sfm *StoreFileMgr) DelStoreFileInfo(fileHashStr string) error {
	sfm.lock.Lock()
	defer sfm.lock.Unlock()
	delete(sfm.fileInfos, fileHashStr)
	err := os.Remove(storeFilePath(fileHashStr))
	if err != nil {
		return err
	}
	files, err := ioutil.ReadDir(STORE_FILES_DIR)
	if err == nil && len(files) == 0 {
		return os.Remove(STORE_FILES_DIR)
	}
	return nil
}

// AddStoredBlock add a new stored block info to map and local storage
func (sfm *StoreFileMgr) AddStoredBlock(fileHashStr, blockHash, server string, nodeList []string) error {
	sfm.lock.Lock()
	defer sfm.lock.Unlock()
	fi, ok := sfm.fileInfos[fileHashStr]
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
	allNodeList := make([]string, 0)
	allNodeList = append(allNodeList, server)
	allNodeList = append(allNodeList, nodeList...)
	newBlk.NodeList = allNodeList
	newBlks = append(newBlks, newBlk)
	fi.Blocks = newBlks
	buf, err := json.Marshal(fi)
	if err != nil {
		fi.Blocks = oldBlks
		return err
	}
	return ioutil.WriteFile(storeFilePath(fileHashStr), buf, 0666)
}

func (sfm *StoreFileMgr) GetStoredBlockNodeList(fileHashStr, blockHashStr string) []string {
	sfm.lock.RLock()
	defer sfm.lock.RUnlock()
	fi, ok := sfm.fileInfos[fileHashStr]
	if !ok || len(fi.Blocks) == 0 {
		return nil
	}
	for _, b := range fi.Blocks {
		if b.Hash == blockHashStr {
			return b.NodeList
		}
	}
	return nil
}

// IsBlockStored check if block has stored
func (sfm *StoreFileMgr) IsBlockStored(fileHashStr string, blockHash string) bool {
	sfm.lock.RLock()
	defer sfm.lock.RUnlock()
	fi, ok := sfm.fileInfos[fileHashStr]
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

func (sfm *StoreFileMgr) StoredBlockCount(fileHashStr string) int {
	sfm.lock.RLock()
	defer sfm.lock.RUnlock()
	fi := sfm.fileInfos[fileHashStr]
	if fi == nil {
		return 0
	}
	return len(fi.Blocks)
}

func (sfm *StoreFileMgr) GetFileProvePrivKey(fileHashStr string) []byte {
	sfm.lock.RLock()
	defer sfm.lock.RUnlock()
	fi := sfm.fileInfos[fileHashStr]
	if fi == nil {
		return nil
	}
	return fi.ProvePrivKey
}

type ReadFileMgr struct {
	readFileInfos map[string]*fmFileInfo
	lock          sync.RWMutex
}

func (rfm *ReadFileMgr) NewReadFile(fileHashStr string) error {
	rfm.lock.Lock()
	defer rfm.lock.Unlock()
	if _, err := os.Stat(DOWNLOAD_FILES_DIR); os.IsNotExist(err) {
		err = os.MkdirAll(DOWNLOAD_FILES_DIR, 0755)
		if err != nil {
			return err
		}
	}

	data, _ := ioutil.ReadFile(downloadingFilePath(fileHashStr))
	if len(data) == 0 {
		rfm.readFileInfos[fileHashStr] = NewFmFileInfo([]byte{})
		return nil
	}

	fi := &fmFileInfo{}
	err := json.Unmarshal(data, fi)
	if err != nil {
		return err
	}
	rfm.readFileInfos[fileHashStr] = fi
	return nil
}

func (rfm *ReadFileMgr) GetReadNodeSliceId(fileHashStr, nodeWalletAddr string) int {
	rfm.lock.RLock()
	defer rfm.lock.RUnlock()
	info := rfm.readFileInfos[fileHashStr]
	cnt := 0
	for _, b := range info.Blocks {
		if b.NodeWalletAddr == nodeWalletAddr {
			cnt++
		}
	}
	return cnt
}

func (rfm *ReadFileMgr) IsBlockRead(fileHashStr, nodeWalletAddr, blockHashStr string) bool {
	rfm.lock.RLock()
	defer rfm.lock.RUnlock()
	info := rfm.readFileInfos[fileHashStr]
	if info == nil {
		return false
	}
	for _, b := range info.Blocks {
		if b.Hash == blockHashStr && b.NodeWalletAddr == nodeWalletAddr {
			return true
		}
	}
	return false
}

func (rfm *ReadFileMgr) GetBlockChilds(fileHashStr, nodeWalletAddr, blockHashStr string) []string {
	rfm.lock.RLock()
	defer rfm.lock.RUnlock()
	info := rfm.readFileInfos[fileHashStr]
	for _, b := range info.Blocks {
		if b.Hash == blockHashStr && b.NodeWalletAddr == nodeWalletAddr {
			return b.ChildHashStrs
		}
	}
	return nil
}

func (rfm *ReadFileMgr) ReceivedBlockFromNode(fileHashStr, blockHashStr, nodeAddr, nodeWalletAddr string, childs []string) {
	rfm.lock.Lock()
	defer rfm.lock.Unlock()

	info := rfm.readFileInfos[fileHashStr]
	if info == nil {
		info = NewFmFileInfo([]byte{})
		info.Blocks = make([]*fmBlockInfo, 0)
	}
	info.Blocks = append(info.Blocks, &fmBlockInfo{
		Hash:           blockHashStr,
		RecvTimestamp:  time.Now().Unix(),
		NodeAddr:       nodeAddr,
		NodeWalletAddr: nodeWalletAddr,
		ChildHashStrs:  childs,
	})
	buf, err := json.Marshal(info)
	if err != nil {
		return
	}
	ioutil.WriteFile(downloadingFilePath(fileHashStr), buf, 0666)
}

func (rfm *ReadFileMgr) RemoveReadFile(fileHashStr string) {
	rfm.lock.Lock()
	defer rfm.lock.Unlock()
	delete(rfm.readFileInfos, fileHashStr)
	os.Remove(downloadingFilePath(fileHashStr))
	files, err := ioutil.ReadDir(DOWNLOAD_FILES_DIR)
	if err == nil && len(files) == 0 {
		os.Remove(DOWNLOAD_FILES_DIR)
	}
}

// GetReadFileNodeWalletAddr get readfile node wallet address
func (rfm *ReadFileMgr) GetReadFileNodeInfo(fileHashStr string) (string, string) {
	rfm.lock.Lock()
	defer rfm.lock.Unlock()
	fi := rfm.readFileInfos[fileHashStr]
	if fi == nil || len(fi.Blocks) == 0 {
		return "", ""
	}
	return fi.Blocks[0].NodeAddr, fi.Blocks[0].NodeWalletAddr
}

func storeFilePath(fileHashStr string) string {
	return STORE_FILES_DIR + fileHashStr
}

func downloadingFilePath(fileHashStr string) string {
	return DOWNLOAD_FILES_DIR + fileHashStr
}
