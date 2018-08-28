package main

import (
	"fmt"
	//"os"
	"os"

	sdk "github.com/daseinio/dasein-go-sdk"
)

var smallTxt = "QmfWAu8auG7NdzVyUAeb1PU5uUs5W3DrCWhMk6B1iCsnvk"
var bigTxt = "QmW5CME8vkw3ndeuDuf5a5oL9x55yPWfhF4fz4R6XTMTBk"

//var largeTxt = "QmU7QRQpSZhukKsraEaa23Re1AzLqpFvyHPwayseVKTbFp"

func testSendFile() {
	client, err := sdk.NewClient()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	var smallFile = "smallfile"

	smallF, err := os.OpenFile(smallFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	defer smallF.Close()
	smallF.WriteString("hello world\n")
	err = client.SendFile(smallFile)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	err = os.Remove(smallFile)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	// var bigFile = "bigfile"
	// bigF, err := os.OpenFile(bigFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	// if err != nil {
	// 	fmt.Println(err.Error())
	// 	return
	// }
	// defer bigF.Close()

	// for i := 1; i < 400000; i++ {
	// 	bigF.WriteString(fmt.Sprintf("%d\n", i))
	// }

	// err = client.SendFile(bigFile)
	// if err != nil {
	// 	fmt.Println(err.Error())
	// 	return
	// }
	// err = os.Remove(bigFile)
	// if err != nil {
	// 	fmt.Println(err.Error())
	// 	return
	// }
}

func testGetData() {

	client, err := sdk.NewClient()
	if err != nil {
		fmt.Println(err.Error())
	}

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
	testSendFile()
}
