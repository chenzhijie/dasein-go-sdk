package dasein_go_sdk

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ontio/ontology/common"
	"github.com/qingche123/dasein-wallet-api/client"
	"github.com/qingche123/dasein-wallet-api/core"
)

const (
	DEFAULT_WALLET_PATH = "./wallet.dat"
	DEFAULT_RPC_ADDR    = "http://localhost:20336"
)

type Setting struct {
	fsGasPrice       uint64
	GasPerKBPerBlock uint64
	gasPerKBForRead  uint64
	gasForChallenge  uint64
}

type ContractRequest struct {
	setting *Setting
	client  *client.DaseinClient
}

func NewContractRequest(wallet, password, rpcSvrAddr string) *ContractRequest {
	if len(wallet) == 0 {
		wallet = DEFAULT_WALLET_PATH
	}
	if len(rpcSvrAddr) == 0 {
		rpcSvrAddr = DEFAULT_RPC_ADDR
	}
	client := client.Init(wallet, password, rpcSvrAddr)
	if client == nil {
		log.Errorf("client is nil")
		return nil
	}
	return &ContractRequest{
		client: client,
	}
}

type StoreFileInfo struct {
	FileHashStr    string
	KeepHours      uint64
	BlockNum       uint64
	BlockSize      uint64
	ChallengeRate  uint64
	ChallengeTimes uint64
	CopyNum        uint64
}

func (cr *ContractRequest) GetNodeList(fileSize uint64, copyNum int32) ([]string, error) {
	infos, err := cr.client.GetNodeList()
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

func (cr *ContractRequest) ProveParamSer(g []byte, g0 []byte, pubKey []byte, fileId []byte, r string) ([]byte, error) {
	return cr.client.ProveParamSer(g, g0, pubKey, fileId, r)
}

func (cr *ContractRequest) PayStoreFile(info *StoreFileInfo, proveParams []byte) ([]byte, error) {
	return cr.client.StoreFile(info.FileHashStr, info.BlockNum, info.BlockSize, info.ChallengeRate, info.ChallengeTimes, info.CopyNum, proveParams)
}

func (cr *ContractRequest) GetFileInfo(fileHashStr string) (*StoreFileInfo, error) {
	info, err := cr.client.GetFileInfo(fileHashStr)
	if err != nil {
		return nil, err
	}
	return &StoreFileInfo{
		FileHashStr:    fileHashStr,
		BlockNum:       info.FileBlockNum,
		BlockSize:      info.FileBlockSize,
		ChallengeRate:  info.ChallengeRate,
		ChallengeTimes: info.ChallengeTimes,
		CopyNum:        info.CopyNum,
	}, nil
}

func (cr *ContractRequest) IsFilePaid(fileHashStr string) (bool, error) {
	info, err := cr.client.GetFileInfo(fileHashStr)
	if err != nil {
		log.Debugf("GetFileInfo error:%s, file:%s", err, fileHashStr)
		return false, nil
	}
	if fmt.Sprintf("%s", info.FileHash) != fileHashStr {
		return false, errors.New("hash is not equal")
	}
	return true, nil
}

type NodeInfo struct {
	Id         string         // peer id string. like `Qmdkh8dBb8p99KGDhazTnNZJpM4hDx95NJtnSLGSKp5tTy`
	Addr       string         // peer full address. like `/ip4/0.0.0.0/tcp/4001/ipfs/Qmdkh8dBb8p99KGDhazTnNZJpM4hDx95NJtnSLGSKp5tTy`
	WalletAddr common.Address // peer wallet address in hash
}

func (cr *ContractRequest) FindStoreFileNodes(fileHashStr string) ([]*NodeInfo, error) {
	details, err := cr.client.GetFileProveDetails(fileHashStr)
	if err != nil {
		// return nil, err
	}
	nodes := make([]*NodeInfo, 0)
	if details != nil {
		for _, d := range details.ProveDetails {
			if len(d.NodeAddr) > 0 {
				nInfo := &NodeInfo{
					Addr:       string(d.NodeAddr),
					WalletAddr: d.WalletAddr,
				}
				nodes = append(nodes, nInfo)
			}
		}
	}

	// for testing
	if len(nodes) == 0 {
		b58Addr, err := common.AddressFromBase58("AYMnqA65pJFKAbbpD8hi5gdNDBmeFBy5hS")
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, &NodeInfo{
			Addr:       "/ip4/127.0.0.1/tcp/4001/ipfs/Qmdkh8dBb8p99KGDhazTnNZJpM4hDx95NJtnSLGSKp5tTy",
			WalletAddr: b58Addr,
		})
		return nodes, nil
	}
	return nodes, nil
}

func (cr *ContractRequest) CalculateReadFee(fileInfo *StoreFileInfo, fileHashStr string) (uint64, error) {
	err := cr.updateSetting()
	if err != nil {
		log.Errorf("update setting failed")
		return 0, err
	}

	readMinFee := fileInfo.BlockNum * fileInfo.BlockSize * cr.setting.fsGasPrice * cr.setting.gasPerKBForRead
	log.Debugf("num:%d, size:%d, gas:%d, read:%d\n", fileInfo.BlockNum, fileInfo.BlockSize, cr.setting.fsGasPrice, cr.setting.gasPerKBForRead)
	return readMinFee, nil
}

func (cr *ContractRequest) PledgeForReadFile(fileHashStr string, nodeWalletAddr common.Address, fee uint64) ([]byte, error) {
	log.Debugf("str:%v, addr:%v, fee:%d", fileHashStr, nodeWalletAddr, fee)
	return cr.client.FsReadFilePledge(fileHashStr, nodeWalletAddr, fee)
}

func (cr *ContractRequest) GenFileReadSettleSlice(fileHashStr string, payTo common.Address, blockNum, blockSize uint64, sliceId uint64) ([]byte, error) {
	if cr.setting == nil {
		err := cr.updateSetting()
		if err != nil {
			return nil, err
		}
	}
	slicePay := blockNum * blockSize * cr.setting.fsGasPrice * cr.setting.gasPerKBForRead
	slice, err := cr.client.GenFileReadSettleSlice([]byte(fileHashStr), payTo, slicePay, sliceId)
	if err != nil {
		return nil, err
	}
	return cr.client.FileReadSettleSliceSer(slice.FileHash, slice.PayFrom, slice.PayTo, slice.SlicePay, slice.SliceId, slice.Sig, slice.PubKey)
}

func (cr *ContractRequest) DeleteFile(fileHashStr string) error {
	return cr.client.DeleteFile(fileHashStr)
}

func (cr *ContractRequest) updateSetting() error {
	core := core.Init(cr.client.WalletPath, string(cr.client.Password), cr.client.OntRpcSrvAddr)
	log.Debugf("pwd :%s", string(cr.client.Password))
	fsSetting, err := core.FsGetSetting()
	if err != nil {
		log.Debugf("get setting error:%s", err)
		return err
	}
	cr.setting = &Setting{
		fsGasPrice:       fsSetting.FsGasPrice,
		GasPerKBPerBlock: fsSetting.GasPerKBPerBlock,
		gasPerKBForRead:  fsSetting.GasPerKBForRead,
		gasForChallenge:  fsSetting.GasForChallenge,
	}
	return nil
}
