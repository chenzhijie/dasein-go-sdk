package main

import (
	"fmt"
	"math/rand"
	"os"

	logging "gx/ipfs/QmRb5jh8z2E8hMGN2tkvs1yHynUanqnZ3UeKwgN1i9P1F8/go-log"

	sdk "github.com/daseinio/dasein-go-sdk"
	"github.com/daseinio/dasein-go-sdk/crypto"
	"github.com/howeyc/gopass"
)

var smallTxt = "QmZyvDNq1gHEkH5USKLUSunZBBya4qox7w4dWGxnt41Zox"
var bigTxt = "QmW5CME8vkw3ndeuDuf5a5oL9x55yPWfhF4fz4R6XTMTBk"

//var largeTxt = "QmU7QRQpSZhukKsraEaa23Re1AzLqpFvyHPwayseVKTbFp"
var deleteTxt = "QmevhnWdtmz89BMXuuX5pSY2uZtqKLz7frJsrCojT5kmb6"

// var node = "/ip4/127.0.0.1/tcp/4001/ipfs/QmR1AqNQBqAjPeLswq86dkJZ5Y7ACVGoXzz2K8tz6MHyUB"
var node = "/ip4/127.0.0.1/tcp/4001/ipfs/Qmdkh8dBb8p99KGDhazTnNZJpM4hDx95NJtnSLGSKp5tTy"

var log = logging.Logger("test")

var encrypt = false
var encryptPassword = "123456"
var wallet = "./wallet.dat"
var rpc = "http://127.0.0.1:20336"

func testSendSmallFile() {
	walletPwd, err := getPassword()
	if err != nil {
		log.Error(err)
	}
	client, err := sdk.NewClient(node, wallet, walletPwd, rpc)
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
	smallF.WriteString("hello world223455\n")
	err = client.SendFile(smallFile, 1, 1, 0, encrypt, encryptPassword)
	if err != nil {
		log.Error(err)
		return
	}
	err = os.Remove(smallFile)
	if err != nil {
		log.Error(err)
		return
	}

}

func testSendBigFile() {
	walletPwd, err := getPassword()
	if err != nil {
		log.Error(err)
	}
	client, err := sdk.NewClient(node, wallet, walletPwd, rpc)
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

	for i := 1; i < 40000; i++ {
		bigF.WriteString(fmt.Sprintf("%d\n", i))
	}
	err = client.SendFile(bigFile, 1, 1, 0, encrypt, encryptPassword)
	if err != nil {
		log.Errorf("send file err:%s", err)
		return
	}
	err = os.Remove(bigFile)
	if err != nil {
		log.Error(err)
		return
	}
}

func testGetData() {
	walletPwd, err := getPassword()
	if err != nil {
		log.Error(err)
	}
	r := sdk.NewContractRequest(wallet, walletPwd, rpc)
	nodes, err := r.FindStoreFileNodes(smallTxt)
	if err != nil {
		log.Error(err)
		return
	}
	randIndx := rand.Intn(len(nodes))
	chosenNode := nodes[randIndx]
	log.Infof("get data from :%s", chosenNode.Addr)
	client, err := sdk.NewClient(chosenNode.Addr, wallet, walletPwd, rpc)
	if err != nil {
		log.Error(err)
		return
	}

	log.Info("-----------------------")
	log.Info("Single Block Test")
	// data, err := client.GetData("QmdYVqoNoRZ9uDbU9FVMkTkmjArhL9u4NWipaNLHMCtAcT", chosenNode.WalletAddr)
	data, err := client.GetData("QmQHkNBsHaaYAQkHrDCYEJSYwtQGXfodMMJhmiEMPc5b4B", chosenNode.WalletAddr)
	// data, err := client.GetData(smallTxt, chosenNode.WalletAddr)
	if err != nil {
		log.Error(err)
	}
	file, err := os.Create("small")
	if err != nil {
		log.Error(err)
	} else {
		log.Infof("GetData %s success", smallTxt)
	}
	_, err = file.Write(data)
	if err != nil {
		log.Error(err)
	}
	file.Close()
	if encrypt {
		crypto.AESDecryptFile("small", encryptPassword, "small-decrypted")
	}
	log.Info("-----------------------")
	// log.Info("Multi Block Test")
	// data, err = client.GetData(bigTxt)
	// if err != nil {
	// 	log.Error(err)
	// }

	// file, err = os.Create("big")
	// if err != nil {
	// 	log.Error(err)
	// }
	// _, err = file.Write(data)
	// if err != nil {
	// 	log.Error(err)
	// }
	// file.Close()

	// log.Info("-----------------------")
	// log.Info("Delete Block Test")
	// err = client.DelData(deleteTxt)
	// if err != nil {
	// 	log.Error(err)
	// } else {
	// 	log.Infof("DelData %s success", deleteTxt)
	// }

	/*
		log.Info("Multi Block Test")
		data, err = client.GetData(largeTxt)
		if err != nil {
			log.Error(err)
		}

		file, err = os.Create("large")
		if err != nil {
			log.Error(err)
		}
		_, err = file.Write(data)
		if err != nil {
			log.Error(err)
		}
		file.Close()
	*/
}

func testDelData() {
	walletPwd, err := getPassword()
	if err != nil {
		log.Error(err)
	}
	r := sdk.NewContractRequest(wallet, walletPwd, rpc)
	nodes, err := r.FindStoreFileNodes(smallTxt)
	if err != nil {
		log.Error(err)
		return
	}
	file := "QmQHkNBsHaaYAQkHrDCYEJSYwtQGXfodMMJhmiEMPc5b4B"

	err = r.DeleteFile(file)
	fmt.Printf("delete file result:%s", err)
	if err != nil {
		log.Error(err)
		return
	}
	// time.Sleep(time.Duration(30) * time.Second)

	for _, node := range nodes {
		client, err := sdk.NewClient(node.Addr, wallet, walletPwd, rpc)
		if err != nil {
			log.Error(err)
			continue
		}
		err = client.DelData(file)
		if err != nil {
			log.Errorf("delete file:%s failed in node:%s", file, node.Addr)
		}
	}
}

func testGetNodeList() {
	walletPwd, err := getPassword()
	if err != nil {
		log.Error(err)
	}
	r := sdk.NewContractRequest(wallet, walletPwd, rpc)
	l, err := r.GetNodeList(0, 1)
	if err != nil {
		log.Error(err)
	}
	log.Infof("list:%v\n", l)
}

func testStoreFile() {
	hashStr := "QmbZdTb7U6eKCmPRjdxBxjmAnAzLvUz2htmTuhqSAVrEKw"
	walletPwd, err := getPassword()
	if err != nil {
		log.Error(err)
	}
	r := sdk.NewContractRequest(wallet, walletPwd, rpc)
	paid, err := r.IsFilePaid(hashStr)
	if err != nil {
		log.Error(err)
	}
	log.Infof("is paid:%t", paid)
	if paid {
		return
	}
	info := &sdk.StoreFileInfo{
		FileHashStr:    hashStr,
		KeepHours:      1,
		BlockNum:       1,
		BlockSize:      256,
		ChallengeRate:  1,
		ChallengeTimes: 1,
		CopyNum:        0,
	}
	h, err := r.PayStoreFile(info, []byte("1"))
	if err != nil {
		log.Error(err)
	}
	log.Infof("list:%x\n", h)
}

func testGetFileProveDetails() {
	walletPwd, err := getPassword()
	if err != nil {
		log.Error(err)
	}
	r := sdk.NewContractRequest(wallet, walletPwd, rpc)
	r.FindStoreFileNodes("QmZyvDNq1gHEkH5USKLUSunZBBya4qox7w4dWGxnt41Zox")
}

func getPassword() (string, error) {
	testing := true
	if testing {
		return "pwd", nil
	} else {
		fmt.Printf("Password:")
		pwd, err := gopass.GetPasswd()
		if err != nil {
			return "", err
		}
		return string(pwd), nil
	}
}

func main() {
	logging.SetLogLevel("test", "DEBUG")
	logging.SetLogLevel("daseingosdk", "DEBUG")
	logging.SetLogLevel("bitswap", "DEBUG")
	// testSendSmallFile()
	// testGetData()
	testDelData()
	// testSendBigFile()
	// testSendSmallFile()
	// testGetNodeList()
	// testGetFileProveDetails()
}
