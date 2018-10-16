package dasein_go_sdk

import (
	"errors"
	"fmt"
	"math"

	"github.com/daseinio/dasein-wallet-api/client"
	"github.com/daseinio/dasein-wallet-api/core"
	"github.com/ontio/ontology/common"
	fs "github.com/ontio/ontology/smartcontract/service/native/ontfs"
)

const (
	DEFAULT_WALLET_PATH = "./wallet.dat"
	DEFAULT_RPC_ADDR    = "http://localhost:20336"
)

type ContractRequest struct {
	setting *fs.FsSetting
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

func (cr *ContractRequest) GetNodeList(fileSize uint64, copyNum int32) ([]string, error) {
	infos, err := cr.client.GetNodeList()
	if err != nil {
		return nil, err
	}
	// TODO: check storage service time is enough
	nodeList := make([]string, 0)
	for _, info := range infos.Item {
		fileSizeInKb := uint64(math.Ceil(float64(fileSize) / 1024.0))
		if info.RestVol < fileSizeInKb {
			continue
		}
		fullAddress := string(info.NodeAddr)
		nodeList = append(nodeList, fullAddress)
	}
	if len(nodeList) < int(copyNum)+1 {
		return nil, fmt.Errorf("nodelist count:%d smaller than copynum:%d", len(nodeList), copyNum)
	}

	return nodeList, nil
}

func (cr *ContractRequest) ProveParamSer(g []byte, g0 []byte, pubKey []byte, fileId []byte, r, pairing string) ([]byte, error) {
	return cr.client.ProveParamSer(g, g0, pubKey, fileId, r, pairing)
}

func (c *ContractRequest) GetFileProveParams(fileProveParam []byte) ([]byte, []byte, []byte, string, string, string, error) {
	core := core.Init(c.client.WalletPath, string(c.client.Password), c.client.OntRpcSrvAddr)
	p, err := core.ProveParamDes(fileProveParam)
	if err != nil {
		return nil, nil, nil, "", "", "", err
	}
	return p.G, p.G0, p.PubKey, string(p.FileId), string(p.R), string(p.Paring), nil
}

func (cr *ContractRequest) PayStoreFile(fileHashStr string, blockNum, blockSize, challengeRate, challengeTimes, copyNum uint64, proveParams []byte) ([]byte, error) {
	return cr.client.StoreFile(fileHashStr, blockNum, blockSize, challengeRate, challengeTimes, copyNum, proveParams)
}

func (cr *ContractRequest) GetFileInfo(fileHashStr string) (*fs.FileInfo, error) {
	return cr.client.GetFileInfo(fileHashStr)
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

func (cr *ContractRequest) GetStoreFileNodes(fileHashStr string) ([]*NodeInfo, error) {
	details, err := cr.client.GetFileProveDetails(fileHashStr)
	if err != nil {
		return nil, err
	}
	if details == nil {
		return nil, errors.New("get details failed, details is nil")
	}
	log.Debugf("get details:%d ,copynum:%d,err:%s\n", details.ProveDetailNum, details.CopyNum, err)
	if details.ProveDetailNum == details.CopyNum+1 {
		return nil, errors.New("all node has finishing prove")
	}
	fileInfo, err := cr.client.GetFileInfo(fileHashStr)
	if err != nil {
		return nil, err
	}
	nodes := make([]*NodeInfo, 0)
	for _, d := range details.ProveDetails {
		log.Debugf("addr:%s, time:%d", string(d.NodeAddr), d.ProveTimes)
		if len(d.NodeAddr) > 0 && d.ProveTimes > 0 && d.ProveTimes < fileInfo.ChallengeTimes {
			nInfo := &NodeInfo{
				Addr:       string(d.NodeAddr),
				WalletAddr: d.WalletAddr,
			}
			nodes = append(nodes, nInfo)
		}
	}
	return nodes, nil
}

// PledgeForReadFile setup pledge for reading file
// plans to download specific blockNum from a node with wallet address
func (cr *ContractRequest) PledgeForReadFile(fileHashStr string, plans map[common.Address]uint64) ([]byte, error) {
	records := make([]fs.Read, 0)
	for address, blockNum := range plans {
		records = append(records, fs.Read{
			ReadAddr:        address,
			MaxReadBlockNum: blockNum,
		})
	}
	readPlan := fs.ReadPlan{
		NodeCount:   uint64(len(plans)),
		NodeRecords: records,
	}
	log.Debugf("str:%v, readPlan:%v", fileHashStr, readPlan)
	return cr.client.FsReadFilePledge(fileHashStr, readPlan)
}

// IsReadFilePledgeExist check if the pledge has commited
func (cr *ContractRequest) IsReadFilePledgeExist(fileHashStr string) bool {
	fpledge, _, err := cr.client.FsGetFileReadPledge(fileHashStr)
	if err != nil {
		log.Debugf("get file read pledge failed:%s", err)
	}
	return fpledge != nil && err == nil
}

func (cr *ContractRequest) GenFileReadSettleSlice(fileHashStr string, payTo common.Address, sliceId uint64) ([]byte, error) {
	if cr.setting == nil {
		err := cr.updateSetting()
		if err != nil {
			return nil, err
		}
	}
	// slicePay := blockNum * blockSize * cr.setting.fsGasPrice * cr.setting.gasPerKBForRead
	slice, err := cr.client.GenFileReadSettleSlice([]byte(fileHashStr), payTo, sliceId)
	if err != nil {
		return nil, err
	}
	return cr.client.FileReadSettleSliceEnc(slice.FileHash, slice.PayFrom, slice.PayTo, slice.SliceId, slice.Sig, slice.PubKey)
}

func (cr *ContractRequest) DeleteFile(fileHashStr string) error {
	return cr.client.DeleteFile(fileHashStr)
}

func (cr *ContractRequest) CancelReadPledge(fileHashStr string) error {
	return cr.client.FsCancelFileRead(fileHashStr)
}

func (cr *ContractRequest) updateSetting() error {
	core := core.Init(cr.client.WalletPath, string(cr.client.Password), cr.client.OntRpcSrvAddr)
	fsSetting, err := core.FsGetSetting()
	if err != nil {
		log.Debugf("get setting error:%s", err)
		return err
	}
	cr.setting = fsSetting
	return nil
}
