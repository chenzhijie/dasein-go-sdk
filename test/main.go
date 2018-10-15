package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	logging "gx/ipfs/QmRb5jh8z2E8hMGN2tkvs1yHynUanqnZ3UeKwgN1i9P1F8/go-log"

	sdk "github.com/daseinio/dasein-go-sdk"
	"github.com/daseinio/dasein-go-sdk/crypto"
)

var smallTxt = "QmZyvDNq1gHEkH5USKLUSunZBBya4qox7w4dWGxnt41Zox"
var bigTxt = "QmW5CME8vkw3ndeuDuf5a5oL9x55yPWfhF4fz4R6XTMTBk"

//var largeTxt = "QmU7QRQpSZhukKsraEaa23Re1AzLqpFvyHPwayseVKTbFp"
var deleteTxt = "QmevhnWdtmz89BMXuuX5pSY2uZtqKLz7frJsrCojT5kmb6"

var log = logging.Logger("test")

var encrypt = false
var encryptPassword = "123456"
var walletPwd = "pwd"
var wallet = "./wallet.dat"
var rpc = "http://127.0.0.1:20336"

func testSendFile(fileName string, rate, times uint64, copyNum int32) {
	client, err := sdk.NewClient("", wallet, walletPwd, rpc)
	if err != nil {
		log.Error(err)
		return
	}
	fileHashStr, err := client.SendFile(fileName, rate, times, copyNum, encrypt, encryptPassword)
	if err != nil {
		log.Error(err)
		return
	}

	// check prove
	r := sdk.NewContractRequest(wallet, walletPwd, rpc)
	retry := 0
	for {
		if retry > sdk.MAX_RETRY_REQUEST_TIMES {
			log.Error("check prove file failed timeout")
			return
		}

		nodes, _ := r.GetStoreFileNodes(fileHashStr)
		if len(nodes) > 0 {
			log.Info("remote nodes have stored file!")
			err = sdk.DelStoreFileInfo(fileHashStr)
			if err != nil {
				log.Error(err)
			}
			break
		}
		retry++
		time.Sleep(time.Duration(sdk.MAX_REQUEST_TIMEWAIT) * time.Second)
	}
}

func testSendSmallFile() {
	client, err := sdk.NewClient("", wallet, walletPwd, rpc)
	if err != nil {
		log.Error(err)
		return
	}
	var smallFile = "smallfile"
	smallF, err := os.OpenFile(smallFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Error(err)
		return
	}
	defer smallF.Close()
	smallF.WriteString("aa123123456s \n")
	fileHashStr, err := client.SendFile(smallFile, 1000, 3, 0, encrypt, encryptPassword)
	if err != nil {
		log.Error(err)
		return
	}
	err = os.Remove(smallFile)
	if err != nil {
		log.Error(err)
		return
	}

	// check prove
	r := sdk.NewContractRequest(wallet, walletPwd, rpc)
	retry := 0
	for {
		if retry > sdk.MAX_RETRY_REQUEST_TIMES {
			log.Error("check prove file failed timeout")
			return
		}
		nodes, _ := r.GetStoreFileNodes(fileHashStr)
		if len(nodes) > 0 {
			log.Info("remote nodes have stored file!")
			err = sdk.DelStoreFileInfo(fileHashStr)
			if err != nil {
				log.Error(err)
			}
			break
		}
		retry++
		time.Sleep(time.Duration(sdk.MAX_REQUEST_TIMEWAIT) * time.Second)
	}

}

func testSendBigFile() {
	client, err := sdk.NewClient("", wallet, walletPwd, rpc)
	if err != nil {
		log.Error(err)
		return
	}
	var bigFile = "bigfile"
	bigF, err := os.OpenFile(bigFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Error(err)
		return
	}
	defer bigF.Close()

	start := 250000
	for i := start; i < start+50000; i++ {
		bigF.WriteString(fmt.Sprintf("%d\n", i))
	}
	log.Debugf("send big file")
	fileHashStr, err := client.SendFile(bigFile, 200, 3, 0, encrypt, encryptPassword)
	if err != nil {
		log.Errorf("send file err:%s", err)
		return
	}
	err = os.Remove(bigFile)
	if err != nil {
		log.Error(err)
		return
	}

	// check prove
	r := sdk.NewContractRequest(wallet, walletPwd, rpc)
	retry := 0
	for {
		if retry > sdk.MAX_RETRY_REQUEST_TIMES {
			log.Error("check prove file failed timeout")
			return
		}
		nodes, _ := r.GetStoreFileNodes(fileHashStr)
		if len(nodes) > 0 {
			log.Info("remote nodes have stored file!")
			err = sdk.DelStoreFileInfo(fileHashStr)
			if err != nil {
				log.Error(err)
			}
			break
		}
		retry++
		time.Sleep(time.Duration(sdk.MAX_REQUEST_TIMEWAIT) * time.Second)
	}
}

func testGetData(fileHashStr string) {
	r := sdk.NewContractRequest(wallet, walletPwd, rpc)
	chosenNode := sdk.GetReadFileNodeInfo(fileHashStr)
	log.Debugf("local chosen node:%v", chosenNode)
	if chosenNode == nil {
		nodes, err := r.GetStoreFileNodes(fileHashStr)
		if err != nil {
			log.Error(err)
			return
		}
		if len(nodes) == 0 {
			log.Errorf("no nodes for:%s", fileHashStr)
			return
		}
		randIndx := rand.Intn(len(nodes))
		chosenNode = nodes[randIndx]
	}

	log.Infof("get data from :%s", chosenNode.Addr)
	client, err := sdk.NewClient(chosenNode.Addr, wallet, walletPwd, rpc)
	if err != nil {
		log.Error(err)
		return
	}

	err = client.GetData(fileHashStr, chosenNode.Addr, chosenNode.WalletAddr)
	if err != nil {
		log.Error(err)
		return
	}
	log.Infof("get: %s success", fileHashStr)
	if encrypt {
		crypto.AESDecryptFile(fileHashStr, encryptPassword, fmt.Sprintf("%s-decrypted", fileHashStr))
	}
}

func testDelData(fileHashStr string) {
	r := sdk.NewContractRequest(wallet, walletPwd, rpc)
	nodes, _ := r.GetStoreFileNodes(fileHashStr)

	err := r.DeleteFile(fileHashStr)
	if err != nil {
		log.Errorf("delete file failed in contract:%s", err)
		return
	}
	retry := 0
	for {
		if retry > sdk.MAX_RETRY_REQUEST_TIMES {
			log.Error("delete file failed timeout")
			return
		}
		info, _ := r.GetFileInfo(fileHashStr)
		if info == nil {
			break
		}
		retry++
		time.Sleep(time.Duration(sdk.MAX_REQUEST_TIMEWAIT) * time.Second)
	}

	err = sdk.DelStoreFileInfo(fileHashStr)
	if err != nil {
		log.Errorf("delete local store file infos failed:%s", err)
	}

	for _, node := range nodes {
		log.Debugf("delete file from node: %v", node.Addr)
		client, err := sdk.NewClient(node.Addr, wallet, walletPwd, rpc)
		if err != nil {
			log.Error(err)
			continue
		}
		err = client.DelData(fileHashStr)
		if err != nil {
			log.Errorf("delete file:%s failed in node:%s, err:%s", fileHashStr, node.Addr, err)
		}
	}
	log.Infof("delete file sucess:%s", fileHashStr)
}

func testByFlags() {
	isEncrypt := flag.Bool("encrypt", false, "Encrypt file")
	ePwd := flag.String("encryptpwd", "", "Encrypt password")
	wPwd := flag.String("walletpwd", "pwd", "Wallet password")
	rates := flag.Int("challengerate", 10, "block count of challenge")
	times := flag.Int("challengetimes", 3, "challenge time")
	copynum := flag.Int("copynum", 0, "backup nodes number for copy")

	if len(os.Args) < 2 {
		return
	}
	operation := os.Args[1]
	var fileName string
	if operation == "send" || operation == "get" || operation == "del" {
		if len(os.Args) < 3 {
			return
		}
		fileName = os.Args[2]
		flag.CommandLine.Parse(os.Args[3:])
	} else {
		flag.CommandLine.Parse(os.Args[2:])
	}

	if len(*ePwd) > 0 {
		encryptPassword = *ePwd
	}
	walletPwd = *wPwd
	encrypt = *isEncrypt
	switch operation {
	case "send":
		testSendFile(fileName, uint64(*rates), uint64(*times), int32(*copynum))
	case "get":
		testGetData(fileName)
	case "del":
		testDelData(fileName)
	case "testsendsmall":
		testSendSmallFile()
	case "testsendbig":
		testSendBigFile()
	}
}

func testGetInfo() {
	r := sdk.NewContractRequest(wallet, walletPwd, rpc)
	fileHashStr := "QmULD2uL21CiH4UdPM6TiXZs2qGiMY5BG2j3ZXTko7FPYJ"
	info, _ := r.GetFileInfo(fileHashStr)
	G, G0, PubKey, FileId, R, Paring, _ := r.GetFileProveParams(info.FileProveParam)
	fmt.Printf("G:\n%v\nG0:\n%v\nPubKey:\n%v\nFileId:\n%v\nR:\n%v\nParing:\n%v\n\n", hex.EncodeToString(G), hex.EncodeToString(G0), hex.EncodeToString(PubKey), FileId, R, Paring)
}

func main() {
	logging.SetLogLevel("test", "DEBUG")
	logging.SetLogLevel("daseingosdk", "DEBUG")
	logging.SetLogLevel("bitswap", "DEBUG")
	testByFlags()
	// testGetInfo()
	// testDelFileAndGet()
	// testSendSmallFile()
	// testGetData("QmafaFyC4DkLcaPTbhBrBxfUWB3BVWNeAnekzzYhgtkztD")
	// testDelData()
	// testSendBigFile()
	// testSendSmallFile()
	// testGetNodeList()
	// testGetFileProveDetails()
}
