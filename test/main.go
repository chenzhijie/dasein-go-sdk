package main

import (
	"fmt"
	//"os"
	sdk "github.com/daseinio/dasein-go-sdk"
	"os"
)

var smallTxt = "QmfWAu8auG7NdzVyUAeb1PU5uUs5W3DrCWhMk6B1iCsnvk"
var bigTxt = "QmW5CME8vkw3ndeuDuf5a5oL9x55yPWfhF4fz4R6XTMTBk"
//var largeTxt = "QmU7QRQpSZhukKsraEaa23Re1AzLqpFvyHPwayseVKTbFp"

func main() {

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
