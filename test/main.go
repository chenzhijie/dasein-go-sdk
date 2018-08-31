package main

import (
	"fmt"
	"os"

	logging "gx/ipfs/QmRb5jh8z2E8hMGN2tkvs1yHynUanqnZ3UeKwgN1i9P1F8/go-log"
	sdk "github.com/daseinio/dasein-go-sdk"
)

var smallTxt = "QmfWAu8auG7NdzVyUAeb1PU5uUs5W3DrCWhMk6B1iCsnvk"
var bigTxt = "QmW5CME8vkw3ndeuDuf5a5oL9x55yPWfhF4fz4R6XTMTBk"

//var largeTxt = "QmU7QRQpSZhukKsraEaa23Re1AzLqpFvyHPwayseVKTbFp"
var deleteTxt = "QmevhnWdtmz89BMXuuX5pSY2uZtqKLz7frJsrCojT5kmb6"

 var server = "/ip4/127.0.0.1/tcp/4001/ipfs/QmR1AqNQBqAjPeLswq86dkJZ5Y7ACVGoXzz2K8tz6MHyUB"
//var server = "/ip4/127.0.0.1/tcp/4001/ipfs/QmTj2ccSejD8eiGj5xEwhEtzkwUvAik1iaQMCheUNQiEng"

var log = logging.Logger("test")

func testSendFile() {

	sdk.Init(server)
	client, err := sdk.NewClient()
	if err != nil {
		log.Error(err)
		return
	}
	// var smallFile = "smallfile"
	// smallF, err := os.OpenFile(smallFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	// if err != nil {
	// 	log.Error(err)
	// 	return
	// }
	// defer smallF.Close()
	// smallF.WriteString("hello world\n")
	// err = client.SendFile(smallFile, "QmTj2ccSejD8eiGj5xEwhEtzkwUvAik1iaQMCheUNQiEng", 2, []string{"Qme2Z9M2FTAyJPDk4BVn2dCv1PoCq4xgsjZDimZKxYyVki", "QmZQgmCeAuFSuxLmbcxLxyXAngKAfcFYzyyuvgQbqVWB9n"})
	// if err != nil {
	// 	log.Error(err)
	// 	return
	// }
	// err = os.Remove(smallFile)
	// if err != nil {
	// 	log.Error(err)
	// 	return
	// }

	var bigFile = "bigfile"
	bigF, err := os.OpenFile(bigFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Error(err)
		return
	}
	defer bigF.Close()

	for i := 1; i < 400000; i++ {
		bigF.WriteString(fmt.Sprintf("%d\n", i))
	}

	err = client.SendFile(bigFile, "QmTj2ccSejD8eiGj5xEwhEtzkwUvAik1iaQMCheUNQiEng", 2, []string{"Qme2Z9M2FTAyJPDk4BVn2dCv1PoCq4xgsjZDimZKxYyVki", "QmZQgmCeAuFSuxLmbcxLxyXAngKAfcFYzyyuvgQbqVWB9n"})
	if err != nil {
		log.Error(err)
		return
	}
	err = os.Remove(bigFile)
	if err != nil {
		log.Error(err)
		return
	}
}

func testGetData() {
	sdk.Init(server)
	client, err := sdk.NewClient()
	if err != nil {
		log.Error(err)
		return
	}

	log.Info("-----------------------")
	log.Info("Single Block Test")
	data, err := client.GetData(smallTxt, "QmR1AqNQBqAjPeLswq86dkJZ5Y7ACVGoXzz2K8tz6MHyUB")
	if err != nil {
		log.Error(err)
	}
	log.Info("GetData: \n", string(data))
	file, err := os.Create("small")
	if err != nil {
		log.Error(err)
	}
	_, err = file.Write(data)
	if err != nil {
		log.Error(err)
	}
	file.Close()
/*
	log.Info("-----------------------")
	log.Info("Multi Block Test")
	data, err = client.GetData(bigTxt, "QmR1AqNQBqAjPeLswq86dkJZ5Y7ACVGoXzz2K8tz6MHyUB")
	if err != nil {
		log.Error(err)
	}

	file, err = os.Create("big")
	if err != nil {
		log.Error(err)
	}
	_, err = file.Write(data)
	if err != nil {
		log.Error(err)
	}
	file.Close()*/

	log.Info("-----------------------")
	log.Info("Delete Block Test")
	err = client.DelData(deleteTxt, "QmR1AqNQBqAjPeLswq86dkJZ5Y7ACVGoXzz2K8tz6MHyUB")
	if err != nil {
		log.Error(err)
	} else {
		log.Infof("Delete %s success", deleteTxt)
	}

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

func main() {
	logging.SetLogLevel("test", "INFO")
	// logging.SetLogLevel("bitswap", "INFO")
	logging.SetLogLevel("daseingosdk", "INFO")
	testGetData()
	//testSendFile()
}
