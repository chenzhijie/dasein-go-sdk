package dasein_go_sdk

import (
	"errors"
	"fmt"
	"strings"

	"github.com/qingche123/dasein-wallet-api/client"
)

type StoreFileInfo struct {
	FileHashStr    string
	KeepHours      uint64
	BlockNum       uint64
	BlockSize      uint64
	ChallengeRate  uint64
	ChallengeTimes uint64
	CopyNum        uint64
}

func GetNodeList(fileSize uint64, copyNum int32, wallet, password, rpcSvrAddr string) ([]string, error) {
	client := client.Init(wallet, password, rpcSvrAddr)
	if client == nil {
		return nil, errors.New("client is nil")
	}
	infos, err := client.GetNodeList()
	if err != nil {
		return nil, err
	}
	// TODO: check storage service time is enough
	nodeList := make([]string, 0)
	for _, info := range infos.Item {
		fullAddress := string(info.NodeAddr)
		parts := strings.Split(fullAddress, "/")
		if len(parts) == 0 {
			continue
		}
		if info.RestVol < fileSize {
			continue
		}
		id := parts[len(parts)-1]
		if len(id) <= 0 {
			continue
		}
		nodeList = append(nodeList, id)
	}
	if len(nodeList) < int(copyNum)+1 {
		return nil, fmt.Errorf("nodelist count:%d smaller than copynum:%d", len(nodeList), copyNum)
	}

	return nodeList, nil
}

func PayStoreFile(info *StoreFileInfo, wallet, password, rpcSvrAddr string) ([]byte, error) {
	client := client.Init(wallet, password, rpcSvrAddr)
	if client == nil {
		return nil, errors.New("client is nil")
	}
	proveBufs := make([]byte, 0)
	return client.StoreFile(info.FileHashStr, info.KeepHours, info.BlockNum, info.BlockSize, info.ChallengeRate, info.ChallengeTimes, info.CopyNum, proveBufs)
}

func IsFilePaid(fileHashStr string, wallet, password, rpcSvrAddr string) (bool, error) {
	client := client.Init(wallet, password, rpcSvrAddr)
	if client == nil {
		return false, errors.New("client is nil")
	}
	info, err := client.GetFileInfo(fileHashStr)
	if err != nil {
		log.Debugf("GetFileInfo error:%s, file:%s", err, fileHashStr)
		return false, nil
	}
	if fmt.Sprintf("%s", info.FileHash) != fileHashStr {
		return false, errors.New("hash is not equal")
	}
	return true, nil
}
