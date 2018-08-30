package main

import (
	"fmt"
	"os"

	sdk "github.com/daseinio/dasein-go-sdk"
)

var smallTxt = "QmfWAu8auG7NdzVyUAeb1PU5uUs5W3DrCWhMk6B1iCsnvk"
var bigTxt = "QmW5CME8vkw3ndeuDuf5a5oL9x55yPWfhF4fz4R6XTMTBk"

//var largeTxt = "QmU7QRQpSZhukKsraEaa23Re1AzLqpFvyHPwayseVKTbFp"
var deleteTxt = "QmevhnWdtmz89BMXuuX5pSY2uZtqKLz7frJsrCojT5kmb6"

var server = "/ip4/127.0.0.1/tcp/4001/ipfs/QmR1AqNQBqAjPeLswq86dkJZ5Y7ACVGoXzz2K8tz6MHyUB"

func testSendFile() {

	sdk.Init(server)
	client, err := sdk.NewClient()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	// var smallFile = "smallfile"
	// smallF, err := os.OpenFile(smallFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	// if err != nil {
	// 	fmt.Println(err.Error())
	// 	return
	// }
	// defer smallF.Close()
	// smallF.WriteString("hello world\n")
	// err = client.SendFile(smallFile, "QmTj2ccSejD8eiGj5xEwhEtzkwUvAik1iaQMCheUNQiEng", 2, []string{"Qme2Z9M2FTAyJPDk4BVn2dCv1PoCq4xgsjZDimZKxYyVki", "QmZQgmCeAuFSuxLmbcxLxyXAngKAfcFYzyyuvgQbqVWB9n"})
	// if err != nil {
	// 	fmt.Println(err.Error())
	// 	return
	// }
	// err = os.Remove(smallFile)
	// if err != nil {
	// 	fmt.Println(err.Error())
	// 	return
	// }

	var bigFile = "bigfile"
	bigF, err := os.OpenFile(bigFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	defer bigF.Close()

	for i := 1; i < 400000; i++ {
		bigF.WriteString(fmt.Sprintf("%d\n", i))
	}

	err = client.SendFile(bigFile, "QmTj2ccSejD8eiGj5xEwhEtzkwUvAik1iaQMCheUNQiEng", 2, []string{"Qme2Z9M2FTAyJPDk4BVn2dCv1PoCq4xgsjZDimZKxYyVki", "QmZQgmCeAuFSuxLmbcxLxyXAngKAfcFYzyyuvgQbqVWB9n"})
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	err = os.Remove(bigFile)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
}

func testGetData() {
	sdk.Init(server)
	client, err := sdk.NewClient()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	fmt.Println("-----------------------")
	fmt.Println("Single Block Test")
	data, err := client.GetData(smallTxt)
	if err != nil {
		fmt.Println(err.Error())
	}

	file, err := os.Create("small")
	if err != nil {
		fmt.Println(err.Error())
	}
	_, err = file.Write(data)
	if err != nil {
		fmt.Println(err.Error())
	}
	file.Close()

	fmt.Println("-----------------------")
	fmt.Println("Multi Block Test")
	data, err = client.GetData(bigTxt)
	if err != nil {
		fmt.Println(err.Error())
	}

	file, err = os.Create("big")
	if err != nil {
		fmt.Println(err.Error())
	}
	_, err = file.Write(data)
	if err != nil {
		fmt.Println(err.Error())
	}
	file.Close()

	fmt.Println("-----------------------")
	fmt.Println("Delete Block Test")
	err = client.DelData(deleteTxt, "QmR1AqNQBqAjPeLswq86dkJZ5Y7ACVGoXzz2K8tz6MHyUB")
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Printf("Delete %s success", deleteTxt)
	}

	/*
		fmt.Println("Multi Block Test")
		data, err = client.GetData(largeTxt)
		if err != nil {
			fmt.Println(err.Error())
		}

		file, err = os.Create("large")
		if err != nil {
			fmt.Println(err.Error())
		}
		_, err = file.Write(data)
		if err != nil {
			fmt.Println(err.Error())
		}
		file.Close()
	*/
}

func main() {
	testGetData()
	// testSendFile()
}
